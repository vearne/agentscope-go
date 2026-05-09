package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/vearne/agentscope-go/pkg/agent"
	"github.com/vearne/agentscope-go/pkg/message"
)

// AgentCard describes an agent for discovery.
type AgentCard struct {
	Name         string            `json:"name"`
	ID           string            `json:"id"`
	Description  string            `json:"description"`
	Endpoint     string            `json:"endpoint"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// A2AServer exposes a local agent over HTTP so remote agents can call it.
type A2AServer struct {
	ag    agent.AgentBase
	card  AgentCard
	srv   *http.Server
	mu    sync.RWMutex
	ready chan struct{}
}

// NewA2AServer creates a server that wraps the given agent.
func NewA2AServer(a agent.AgentBase, card AgentCard) *A2AServer {
	return &A2AServer{
		ag:    a,
		card:  card,
		ready: make(chan struct{}),
	}
}

// Start launches the HTTP server on the given address (e.g. ":8080").
func (s *A2AServer) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/card", s.handleCard)
	mux.HandleFunc("/a2a/reply", s.handleReply)
	mux.HandleFunc("/a2a/observe", s.handleObserve)

	s.mu.Lock()
	s.srv = &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	s.mu.Unlock()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	close(s.ready)

	go func() {
		if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("[A2A] server exited: %v", err)
		}
	}()

	return nil
}

// Ready returns a channel that is closed once the server is listening.
func (s *A2AServer) Ready() <-chan struct{} {
	return s.ready
}

// Stop performs a graceful shutdown.
func (s *A2AServer) Stop(ctx context.Context) error {
	s.mu.RLock()
	srv := s.srv
	s.mu.RUnlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

func (s *A2AServer) handleCard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.card)
}

func (s *A2AServer) handleReply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("read body: %v", err), http.StatusBadRequest)
		return
	}
	defer func() { _ = r.Body.Close() }()

	var msg message.Msg
	if unmarshalErr := json.Unmarshal(body, &msg); unmarshalErr != nil {
		http.Error(w, fmt.Sprintf("unmarshal: %v", unmarshalErr), http.StatusBadRequest)
		return
	}

	resp, err := s.ag.Reply(r.Context(), &msg)
	if err != nil {
		http.Error(w, fmt.Sprintf("agent reply: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *A2AServer) handleObserve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("read body: %v", err), http.StatusBadRequest)
		return
	}
	defer func() { _ = r.Body.Close() }()

	var msg message.Msg
	if err := json.Unmarshal(body, &msg); err != nil {
		http.Error(w, fmt.Sprintf("unmarshal: %v", err), http.StatusBadRequest)
		return
	}

	if err := s.ag.Observe(r.Context(), &msg); err != nil {
		http.Error(w, fmt.Sprintf("agent observe: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// A2AClient calls remote agents over HTTP. It implements agent.AgentBase
// so it can participate in pipelines transparently.
type A2AClient struct {
	endpoint   string
	httpClient *http.Client
	card       *AgentCard
	mu         sync.RWMutex
}

// NewA2AClient creates a client pointing at the given base endpoint
// (e.g. "http://localhost:8080").
func NewA2AClient(endpoint string) *A2AClient {
	return &A2AClient{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Discover fetches the remote agent card and caches it locally.
func (c *A2AClient) Discover(ctx context.Context) error {
	card, err := c.GetCard(ctx)
	if err != nil {
		return fmt.Errorf("discover agent at %s: %w", c.endpoint, err)
	}
	c.mu.Lock()
	c.card = card
	c.mu.Unlock()
	return nil
}

// GetCard retrieves the remote agent's card.
func (c *A2AClient) GetCard(ctx context.Context) (*AgentCard, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+"/a2a/card", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get card: %d %s", resp.StatusCode, string(body))
	}

	var card AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("decode card: %w", err)
	}
	return &card, nil
}

// Reply sends a message to the remote agent and returns its response.
func (c *A2AClient) Reply(ctx context.Context, msg *message.Msg) (*message.Msg, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/a2a/reply", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("reply: %d %s", resp.StatusCode, string(respBody))
	}

	var result message.Msg
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// Observe sends an observation message to the remote agent.
func (c *A2AClient) Observe(ctx context.Context, msg *message.Msg) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/a2a/observe", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("observe: %d %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// Name returns the cached agent name or a default.
func (c *A2AClient) Name() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.card != nil {
		return c.card.Name
	}
	return "remote-agent"
}

// ID returns the cached agent ID or an empty string.
func (c *A2AClient) ID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.card != nil {
		return c.card.ID
	}
	return ""
}

func (c *A2AClient) Interrupt() {}

func (c *A2AClient) HandleInterrupt(_ context.Context, _ *message.Msg) (*message.Msg, error) {
	return nil, fmt.Errorf("A2AClient does not support interrupt handling")
}

// Compile-time check that A2AClient implements agent.AgentBase.
var _ agent.AgentBase = (*A2AClient)(nil)

// A2ABus is an in-process registry for discovering remote agents.
type A2ABus struct {
	agents map[string]*AgentCard
	mu     sync.RWMutex
}

// NewA2ABus creates a new agent registry.
func NewA2ABus() *A2ABus {
	return &A2ABus{
		agents: make(map[string]*AgentCard),
	}
}

// Register adds an agent card to the bus.
func (b *A2ABus) Register(card AgentCard) {
	b.mu.Lock()
	defer b.mu.Unlock()
	cp := card
	b.agents[card.ID] = &cp
}

// Deregister removes an agent from the bus by ID.
func (b *A2ABus) Deregister(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.agents, id)
}

// Get returns the card for the given agent ID.
func (b *A2ABus) Get(id string) (*AgentCard, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	card, ok := b.agents[id]
	if !ok {
		return nil, false
	}
	cp := *card
	return &cp, true
}

// List returns all registered agent cards.
func (b *A2ABus) List() []AgentCard {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]AgentCard, 0, len(b.agents))
	for _, card := range b.agents {
		result = append(result, *card)
	}
	return result
}
