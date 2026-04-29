# Memory Module

The memory module provides various backends for storing and managing conversation messages and long-term memory in agentscope-go.

## Working Memory

Working memory stores recent conversation messages with support for marks (tags) for filtering and organization.

### Interfaces

```go
type MemoryBase interface {
    // Basic operations
    Add(ctx context.Context, msgs ...*message.Msg) error
    AddWithMarks(ctx context.Context, msgs []*message.Msg, marks []string) error
    GetMessages() []*message.Msg
    GetMemory(ctx context.Context, mark string, excludeMark string, prependSummary bool) ([]*message.Msg, error)
    Clear(ctx context.Context) error
    Size() int
    ToStrList() []string

    // Mark-based operations
    Delete(ctx context.Context, msgIDs []string) (int, error)
    DeleteByMark(ctx context.Context, marks []string) (int, error)
    UpdateCompressedSummary(ctx context.Context, summary string) error
    UpdateMessagesMark(ctx context.Context, newMark string, oldMark string, msgIDs []string) (int, error)
}
```

### Implementations

#### 1. InMemoryMemory

In-memory storage using Go's sync.RWMutex for thread-safe operations.

```go
import "github.com/vearne/agentscope-go/pkg/memory"

mem := memory.NewInMemoryMemory()

// Add messages
msg1 := message.NewMsg("user", "Hello!", "user")
msg2 := message.NewMsg("assistant", "Hi there!", "assistant")
mem.Add(context.Background(), msg1, msg2)

// Add with marks
mem.AddWithMarks(context.Background(), []*message.Msg{msg1}, []string{"important", "greeting"})

// Get all messages
msgs := mem.GetMessages()

// Get messages with mark filtering
importantMsgs, _ := mem.GetMemory(context.Background(), "important", "", false)

// Update marks
mem.UpdateMessagesMark(context.Background(), "priority", "", nil)

// Delete messages
mem.Delete(context.Background(), []string{msg1.ID})

// Delete by mark
mem.DeleteByMark(context.Background(), []string{"greeting"})
```

#### 2. RedisMemory

Redis-based storage with session and user context isolation.

```go
import (
    "context"
    "github.com/redis/go-redis/v9"
    "github.com/vearne/agentscope-go/pkg/memory"
)

client := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

mem := memory.NewRedisMemory(
    client,
    memory.WithSessionID("session-123"),
    memory.WithRedisUserID("user-456"),
    memory.WithKeyPrefix("myapp:"),
    memory.WithKeyTTL(24*time.Hour),
)

// Usage is the same as InMemoryMemory
```

#### 3. SQLMemory

SQL-based storage supporting SQLite, MySQL, and PostgreSQL.

```go
import (
    "database/sql"
    _ "github.com/mattn/go-sqlite3"
    "github.com/vearne/agentscope-go/pkg/memory"
)

db, _ := sql.Open("sqlite3", "memory.db")
defer db.Close()

mem := memory.NewSQLMemory(
    db,
    memory.WithSQLSessionID("session-123"),
    memory.WithSQLUserID("user-456"),
)

// Usage is the same as InMemoryMemory
```

## Long Term Memory

Long-term memory provides persistent storage and retrieval of information over extended periods with semantic search capabilities.

### Interface

```go
type LongTermMemoryBase interface {
    // Developer methods
    Record(ctx context.Context, msgs []*message.Msg) (interface{}, error)
    Retrieve(ctx context.Context, msg *message.Msg, limit int) (string, error)

    // Tool methods for agents
    RecordToMemory(ctx context.Context, thinking string, content []string) (*tool.ToolResponse, error)
    RetrieveFromMemory(ctx context.Context, keywords []string, limit int) (*tool.ToolResponse, error)
}
```

### Mem0LongTermMemory

A simplified implementation of long-term memory inspired by mem0.ai, with local storage and basic semantic search using Jaccard similarity.

```go
import (
    "context"
    "github.com/vearne/agentscope-go/pkg/memory"
)

ltm := memory.NewMem0LongTermMemory(
    memory.WithAgentID("my-agent"),
    memory.WithMem0UserID("user-123"),
    memory.WithRunID("session-456"),
)

// Record information
msg := message.NewMsg("user", "John is a software engineer", "user")
ltm.Record(context.Background(), []*message.Msg{msg})

// Retrieve information
query := message.NewMsg("user", "What do you know about John?", "user")
results, _ := ltm.Retrieve(context.Background(), query, 5)

// Tool methods (for use by agents)
response, _ := ltm.RecordToMemory(
    context.Background(),
    "User mentioned their job",
    []string{"User is a software engineer", "User works at TechCorp"},
)

response, _ = ltm.RetrieveFromMemory(
    context.Background(),
    []string{"software engineer", "TechCorp"},
    5,
)

// Export/Import
json, _ := ltm.ToJSON()
ltm.FromJSON(json)

// Get statistics
count := ltm.GetMemoryCount()

// Clear all memories
ltm.ClearAll()
```

## Using Memory with Agents

### Working Memory

```go
import (
    "context"
    "github.com/vearne/agentscope-go/pkg/agent"
    "github.com/vearne/agentscope-go/pkg/memory"
    "github.com/vearne/agentscope-go/pkg/model"
)

// Create memory backend
mem := memory.NewInMemoryMemory()

// Create agent with memory
ag := agent.NewReActAgent(
    agent.WithReActName("assistant"),
    agent.WithReActModel(model),
    agent.WithReActMemory(mem),
)

// Agent automatically uses memory for conversation history
response, _ := ag.Reply(context.Background(), userMessage)
```

### Long Term Memory

```go
import (
    "context"
    "github.com/vearne/agentscope-go/pkg/agent"
    "github.com/vearne/agentscope-go/pkg/memory"
    "github.com/vearne/agentscope-go/pkg/model"
    "github.com/vearne/agentscope-go/pkg/tool"
)

// Create long-term memory
ltm := memory.NewMem0LongTermMemory(
    memory.WithAgentID("assistant"),
    memory.WithMem0UserID("user-123"),
)

// Create toolkit with long-term memory tools
tk := tool.NewToolkit()
tk.Register("record_to_memory", "Record important information to long-term memory", nil, func(ctx context.Context, args map[string]interface{}) (*tool.ToolResponse, error) {
    thinking := args["thinking"].(string)
    content := args["content"].([]string)
    return ltm.RecordToMemory(ctx, thinking, content)
})

tk.Register("retrieve_from_memory", "Retrieve information from long-term memory", nil, func(ctx context.Context, args map[string]interface{}) (*tool.ToolResponse, error) {
    keywords := args["keywords"].([]string)
    limit := 5
    if l, ok := args["limit"].(int); ok {
        limit = l
    }
    return ltm.RetrieveFromMemory(ctx, keywords, limit)
})

// Create agent with toolkit
ag := agent.NewReActAgent(
    agent.WithReActName("assistant"),
    agent.WithReActModel(model),
    agent.WithReActToolkit(tk),
)
```

## Memory Features

### Marks (Tags)

Marks allow you to tag and filter messages:

```go
// Add messages with marks
mem.AddWithMarks(ctx, msgs, []string{"important", "urgent"})

// Filter by mark
importantMsgs, _ := mem.GetMemory(ctx, "important", "", false)

// Exclude marks
nonUrgentMsgs, _ := mem.GetMemory(ctx, "", "urgent", false)

// Update marks
mem.UpdateMessagesMark(ctx, "priority", "important", nil)

// Remove marks
mem.UpdateMessagesMark(ctx, "", "urgent", nil)
```

### Compressed Summary

Store a compressed summary of the conversation:

```go
mem.UpdateCompressedSummary(ctx, "User is planning a vacation to Japan")

// Retrieve with summary prepended
msgs, _ := mem.GetMemory(ctx, "", "", true)
// First message will be the summary
```

### Message Deletion

Delete messages by ID or mark:

```go
// Delete specific messages
mem.Delete(ctx, []string{"msg-id-1", "msg-id-2"})

// Delete all messages with a mark
mem.DeleteByMark(ctx, []string{"temp"})
```

## Persistence

### RedisMemory Persistence

RedisMemory automatically persists to Redis with optional TTL:

```go
mem := memory.NewRedisMemory(
    client,
    memory.WithKeyTTL(24 * time.Hour), // Keys expire after 24 hours
)
```

### SQLMemory Persistence

SQLMemory persists to the database:

```go
db, _ := sql.Open("sqlite3", "conversations.db")
mem := memory.NewSQLMemory(db)
// All operations are automatically persisted
```

### Mem0LongTermMemory Persistence

Export/import for Mem0LongTermMemory:

```go
// Export
json, _ := ltm.ToJSON()
os.WriteFile("memories.json", []byte(json), 0644)

// Import
data, _ := os.ReadFile("memories.json")
ltm.FromJSON(string(data))
```

## Performance Considerations

- **InMemoryMemory**: Fastest, but not persistent. Suitable for short-lived sessions.
- **RedisMemory**: Good performance with persistence. Requires Redis server.
- **SQLMemory**: Good for complex queries and persistence. May be slower than Redis for simple operations.
- **Mem0LongTermMemory**: Uses in-memory storage by default. For production, consider adding a vector database backend.

## Thread Safety

All memory implementations are thread-safe:
- InMemoryMemory uses sync.RWMutex
- RedisMemory relies on Redis's atomic operations
- SQLMemory uses database transactions

## Best Practices

1. **Choose the right backend**:
   - Use InMemoryMemory for testing and short-lived sessions
   - Use RedisMemory for distributed systems
   - Use SQLMemory for complex queries and analytics

2. **Use marks effectively**:
   - Tag important messages for easy retrieval
   - Use hierarchical marks like "category/subcategory"
   - Clean up temporary marks after use

3. **Manage memory size**:
   - Use DeleteByMark to remove old messages
   - Implement periodic cleanup for long-running sessions
   - Consider memory limits for InMemoryMemory

4. **Long-term memory**:
   - Record only essential information
   - Use meaningful keywords for retrieval
   - Regularly export and backup memory data
