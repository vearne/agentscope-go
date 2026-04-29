package memory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/tool"
)

// Mem0LongTermMemory is a simplified Go implementation of long-term memory
// inspired by mem0.ai, with local storage and basic semantic search.
type Mem0LongTermMemory struct {
	agentID  string
	userID   string
	runID    string
	memories []memoryRecord
	mu       sync.RWMutex
}

// memoryRecord represents a single memory entry.
type memoryRecord struct {
	ID         string    `json:"id"`
	Content    string    `json:"content"`
	Metadata   metadata  `json:"metadata"`
	MemoryType string    `json:"memory_type"`
	CreatedAt  time.Time `json:"created_at"`
}

// metadata contains contextual information for the memory.
type metadata struct {
	AgentID string `json:"agent_id,omitempty"`
	UserID  string `json:"user_id,omitempty"`
	RunID   string `json:"run_id,omitempty"`
}

// Mem0Option is a function type for configuring Mem0LongTermMemory.
type Mem0Option func(*Mem0LongTermMemory)

// WithAgentID sets the agent ID for the memory.
func WithAgentID(agentID string) Mem0Option {
	return func(m *Mem0LongTermMemory) {
		m.agentID = agentID
	}
}

// WithMem0UserID sets the user ID for the memory.
func WithMem0UserID(userID string) Mem0Option {
	return func(m *Mem0LongTermMemory) {
		m.userID = userID
	}
}

// WithRunID sets the run ID for the memory.
func WithRunID(runID string) Mem0Option {
	return func(m *Mem0LongTermMemory) {
		m.runID = runID
	}
}

// NewMem0LongTermMemory creates a new Mem0LongTermMemory instance.
func NewMem0LongTermMemory(opts ...Mem0Option) *Mem0LongTermMemory {
	m := &Mem0LongTermMemory{
		memories: make([]memoryRecord, 0),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// Record records information from messages to long-term memory.
func (m *Mem0LongTermMemory) Record(ctx context.Context, msgs []*message.Msg) (interface{}, error) {
	if len(msgs) == 0 {
		return nil, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var results []interface{}

	for _, msg := range msgs {
		content := msg.GetTextContent()
		if content == "" {
			continue
		}

		record := memoryRecord{
			ID:        generateID(),
			Content:   content,
			MemoryType: "semantic",
			CreatedAt: time.Now(),
			Metadata: metadata{
				AgentID: m.agentID,
				UserID:  m.userID,
				RunID:   m.runID,
			},
		}

		m.memories = append(m.memories, record)
		results = append(results, map[string]interface{}{
			"id":      record.ID,
			"content": record.Content,
		})
	}

	return results, nil
}

// Retrieve retrieves information from long-term memory based on the input message.
func (m *Mem0LongTermMemory) Retrieve(ctx context.Context, msg *message.Msg, limit int) (string, error) {
	if msg == nil {
		return "", nil
	}

	query := msg.GetTextContent()
	if query == "" {
		return "", nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	scores := m.calculateSimilarityScores(query)

	topMemories := m.getTopMemories(scores, limit)

	var results []string
	for _, mem := range topMemories {
		results = append(results, mem.Content)
	}

	return strings.Join(results, "\n"), nil
}

// RecordToMemory is a tool function for agents to record important information.
func (m *Mem0LongTermMemory) RecordToMemory(ctx context.Context, thinking string, content []string) (*tool.ToolResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var fullContent strings.Builder
	if thinking != "" {
		fullContent.WriteString(thinking)
		fullContent.WriteString("\n")
	}
	fullContent.WriteString(strings.Join(content, "\n"))

	record := memoryRecord{
		ID:        generateID(),
		Content:   fullContent.String(),
		MemoryType: "semantic",
		CreatedAt: time.Now(),
		Metadata: metadata{
			AgentID: m.agentID,
			UserID:  m.userID,
			RunID:   m.runID,
		},
	}

	m.memories = append(m.memories, record)

	return tool.NewToolResponse(fmt.Sprintf("Successfully recorded content to memory. ID: %s", record.ID)), nil
}

// RetrieveFromMemory retrieves memory based on keywords.
func (m *Mem0LongTermMemory) RetrieveFromMemory(ctx context.Context, keywords []string, limit int) (*tool.ToolResponse, error) {
	if len(keywords) == 0 {
		return tool.NewToolResponse("No keywords provided"), nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	query := strings.Join(keywords, " ")
	scores := m.calculateSimilarityScores(query)
	topMemories := m.getTopMemories(scores, limit)

	if len(topMemories) == 0 {
		return tool.NewToolResponse("No memories found matching the keywords"), nil
	}

	var results []string
	for _, mem := range topMemories {
		results = append(results, mem.Content)
	}

	return tool.NewToolResponse(strings.Join(results, "\n")), nil
}

// calculateSimilarityScores calculates similarity scores for all memories against a query.
func (m *Mem0LongTermMemory) calculateSimilarityScores(query string) []memoryScore {
	var scores []memoryScore

	queryTokens := tokenize(query)

	for _, mem := range m.memories {
		memTokens := tokenize(mem.Content)
		score := calculateJaccardSimilarity(queryTokens, memTokens)

		if score > 0 {
			scores = append(scores, memoryScore{
				record: mem,
				score:  score,
			})
		}
	}

	return scores
}

// getTopMemories returns the top N memories by similarity score.
func (m *Mem0LongTermMemory) getTopMemories(scores []memoryScore, limit int) []memoryRecord {
	if limit <= 0 {
		return []memoryRecord{}
	}

	if len(scores) <= limit {
		records := make([]memoryRecord, len(scores))
		for i, score := range scores {
			records[i] = score.record
		}
		return records
	}

	quickSelectScores(scores, 0, len(scores)-1, limit)

	records := make([]memoryRecord, limit)
	for i := 0; i < limit; i++ {
		records[i] = scores[i].record
	}

	return records
}

// memoryScore represents a memory with its similarity score.
type memoryScore struct {
	record memoryRecord
	score  float64
}

// tokenize splits text into lowercase tokens.
func tokenize(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	return words
}

// calculateJaccardSimilarity calculates Jaccard similarity between two token sets.
func calculateJaccardSimilarity(tokens1, tokens2 []string) float64 {
	if len(tokens1) == 0 || len(tokens2) == 0 {
		return 0
	}

	set1 := make(map[string]struct{})
	for _, token := range tokens1 {
		set1[token] = struct{}{}
	}

	set2 := make(map[string]struct{})
	for _, token := range tokens2 {
		set2[token] = struct{}{}
	}

	intersection := 0
	for token := range set1 {
		if _, exists := set2[token]; exists {
			intersection++
		}
	}

	union := len(set1) + len(set2) - intersection

	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

// quickSelectScores performs quickselect to find top N scores.
func quickSelectScores(scores []memoryScore, left, right, k int) {
	if left >= right {
		return
	}

	pivotIndex := partition(scores, left, right)

	if k-1 == pivotIndex {
		return
	} else if k-1 < pivotIndex {
		quickSelectScores(scores, left, pivotIndex-1, k)
	} else {
		quickSelectScores(scores, pivotIndex+1, right, k)
	}
}

// partition partitions the scores array.
func partition(scores []memoryScore, left, right int) int {
	pivot := scores[right]
	i := left

	for j := left; j < right; j++ {
		if scores[j].score > pivot.score {
			scores[i], scores[j] = scores[j], scores[i]
			i++
		}
	}

	scores[i], scores[right] = scores[right], scores[i]
	return i
}

// generateID generates a unique ID.
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// ToJSON exports memories to JSON format.
func (m *Mem0LongTermMemory) ToJSON() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.MarshalIndent(m.memories, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// FromJSON imports memories from JSON format.
func (m *Mem0LongTermMemory) FromJSON(jsonData string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var memories []memoryRecord
	if err := json.Unmarshal([]byte(jsonData), &memories); err != nil {
		return err
	}

	m.memories = memories
	return nil
}

// GetMemoryCount returns the number of memories stored.
func (m *Mem0LongTermMemory) GetMemoryCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.memories)
}

// ClearAll clears all memories.
func (m *Mem0LongTermMemory) ClearAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.memories = make([]memoryRecord, 0)
}
