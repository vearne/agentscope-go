package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/vearne/agentscope-go/pkg/message"
)

type mockAgent struct {
	name string
	id   string
}

func (m *mockAgent) Reply(_ context.Context, msg *message.Msg) (*message.Msg, error) {
	return message.NewMsg(m.name, "echo: "+msg.GetTextContent(), "assistant"), nil
}

func (m *mockAgent) Observe(_ context.Context, _ *message.Msg) error {
	return nil
}

func (m *mockAgent) Name() string { return m.name }
func (m *mockAgent) ID() string   { return m.id }

func (m *mockAgent) Interrupt() {}

func (m *mockAgent) HandleInterrupt(_ context.Context, _ *message.Msg) (*message.Msg, error) {
	return nil, fmt.Errorf("mockAgent does not support interrupt handling")
}

func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("get free port: %v", err)
	}
	_ = ln.Close()
	return ln.Addr().String()
}

func TestA2AServer_Reply(t *testing.T) {
	addr := freePort(t)
	ma := &mockAgent{name: "test-agent", id: "agent-1"}
	card := AgentCard{
		Name:        "test-agent",
		ID:          "agent-1",
		Description: "test",
		Endpoint:    "http://" + addr,
	}

	srv := NewA2AServer(ma, card)
	if err := srv.Start(addr); err != nil {
		t.Fatalf("start server: %v", err)
	}
	<-srv.Ready()
	defer func() { _ = srv.Stop(context.Background()) }()

	client := NewA2AClient("http://" + addr)

	msg := message.NewMsg("user", "hello", "user")
	resp, err := client.Reply(context.Background(), msg)
	if err != nil {
		t.Fatalf("reply: %v", err)
	}

	expected := "echo: hello"
	if resp.GetTextContent() != expected {
		t.Errorf("expected '%s', got '%s'", expected, resp.GetTextContent())
	}
}

func TestA2AServer_GetCard(t *testing.T) {
	addr := freePort(t)
	ma := &mockAgent{name: "card-agent", id: "agent-2"}
	card := AgentCard{
		Name:        "card-agent",
		ID:          "agent-2",
		Description: "card test",
		Endpoint:    "http://" + addr,
		Capabilities: []string{"reply", "observe"},
	}

	srv := NewA2AServer(ma, card)
	if err := srv.Start(addr); err != nil {
		t.Fatalf("start server: %v", err)
	}
	<-srv.Ready()
	defer func() { _ = srv.Stop(context.Background()) }()

	client := NewA2AClient("http://" + addr)
	gotCard, err := client.GetCard(context.Background())
	if err != nil {
		t.Fatalf("get card: %v", err)
	}

	if gotCard.Name != "card-agent" {
		t.Errorf("expected name 'card-agent', got '%s'", gotCard.Name)
	}
	if gotCard.ID != "agent-2" {
		t.Errorf("expected id 'agent-2', got '%s'", gotCard.ID)
	}
	if len(gotCard.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(gotCard.Capabilities))
	}
}

func TestA2AServer_Observe(t *testing.T) {
	addr := freePort(t)
	ma := &mockAgent{name: "obs-agent", id: "agent-3"}
	card := AgentCard{Name: "obs-agent", ID: "agent-3", Endpoint: "http://" + addr}

	srv := NewA2AServer(ma, card)
	if err := srv.Start(addr); err != nil {
		t.Fatalf("start server: %v", err)
	}
	<-srv.Ready()
	defer func() { _ = srv.Stop(context.Background()) }()

	client := NewA2AClient("http://" + addr)
	msg := message.NewMsg("user", "observe this", "user")
	if err := client.Observe(context.Background(), msg); err != nil {
		t.Fatalf("observe: %v", err)
	}
}

func TestA2AClient_Discover(t *testing.T) {
	addr := freePort(t)
	ma := &mockAgent{name: "disco-agent", id: "agent-4"}
	card := AgentCard{
		Name:        "disco-agent",
		ID:          "agent-4",
		Description: "discoverable",
		Endpoint:    "http://" + addr,
	}

	srv := NewA2AServer(ma, card)
	if err := srv.Start(addr); err != nil {
		t.Fatalf("start server: %v", err)
	}
	<-srv.Ready()
	defer func() { _ = srv.Stop(context.Background()) }()

	client := NewA2AClient("http://" + addr)
	if client.Name() != "remote-agent" {
		t.Errorf("expected default name before discover, got '%s'", client.Name())
	}

	if err := client.Discover(context.Background()); err != nil {
		t.Fatalf("discover: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	if client.Name() != "disco-agent" {
		t.Errorf("expected 'disco-agent' after discover, got '%s'", client.Name())
	}
	if client.ID() != "agent-4" {
		t.Errorf("expected 'agent-4' after discover, got '%s'", client.ID())
	}
}

func TestA2ABus_RegisterAndList(t *testing.T) {
	bus := NewA2ABus()

	card1 := AgentCard{Name: "a1", ID: "id-1", Endpoint: "http://localhost:8080"}
	card2 := AgentCard{Name: "a2", ID: "id-2", Endpoint: "http://localhost:8081"}

	bus.Register(card1)
	bus.Register(card2)

	list := bus.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(list))
	}

	got, ok := bus.Get("id-1")
	if !ok {
		t.Fatal("expected to find id-1")
	}
	if got.Name != "a1" {
		t.Errorf("expected name 'a1', got '%s'", got.Name)
	}
}

func TestA2ABus_Deregister(t *testing.T) {
	bus := NewA2ABus()
	bus.Register(AgentCard{Name: "a1", ID: "id-1"})
	bus.Register(AgentCard{Name: "a2", ID: "id-2"})

	bus.Deregister("id-1")

	if _, ok := bus.Get("id-1"); ok {
		t.Fatal("expected id-1 to be removed")
	}
	if len(bus.List()) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(bus.List()))
	}
}

func TestA2AClient_ImplementsAgentBase(t *testing.T) {
	// This test exists purely to verify compile-time interface satisfaction.
	// The var _ check in a2a.go handles this, but an explicit test is nice.
	client := NewA2AClient("http://localhost:9999")
	_ = client // just ensure construction works
}

func TestA2AServer_CardJSON(t *testing.T) {
	card := AgentCard{
		Name:         "test",
		ID:           "123",
		Description:  "desc",
		Endpoint:     "http://localhost:8080",
		Capabilities: []string{"reply"},
		Metadata:     map[string]string{"foo": "bar"},
	}

	data, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("marshal card: %v", err)
	}

	var decoded AgentCard
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal card: %v", err)
	}

	if decoded.Name != card.Name {
		t.Errorf("name mismatch: %s != %s", decoded.Name, card.Name)
	}
	if decoded.Metadata["foo"] != "bar" {
		t.Errorf("metadata mismatch: %v", decoded.Metadata)
	}
}
