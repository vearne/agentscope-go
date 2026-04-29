package agent

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type recordedSpan struct {
	name  string
	attrs map[string]attribute.Value
}

type recordingExporter struct {
	mu    sync.Mutex
	spans []recordedSpan
}

func (e *recordingExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, s := range spans {
		rec := recordedSpan{name: s.Name(), attrs: map[string]attribute.Value{}}
		for _, kv := range s.Attributes() {
			rec.attrs[string(kv.Key)] = kv.Value
		}
		e.spans = append(e.spans, rec)
	}
	return nil
}

func (e *recordingExporter) Shutdown(_ context.Context) error { return nil }

// TestReActAgent_TraceAttributes verifies that the chat / format spans carry
// gen_ai.input.messages and gen_ai.output.messages attributes that
// agentscope-studio uses to render the Span Input/Output panels.
func TestReActAgent_TraceAttributes(t *testing.T) {
	exp := &recordingExporter{}
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(prev)

	mockM := &mockModel{
		responses: []*model.ChatResponse{
			model.NewChatResponse([]message.ContentBlock{
				message.NewTextBlock("hello from mock"),
			}),
		},
	}

	ag := NewReActAgent(
		WithReActName("tester"),
		WithReActModel(mockM),
		WithReActFormatter(&mockFormatter{}),
	)

	if _, err := ag.Reply(context.Background(), NewUserMsg("user", "ping")); err != nil {
		t.Fatalf("Reply failed: %v", err)
	}

	if len(exp.spans) == 0 {
		t.Fatalf("no spans were exported")
	}

	var invoke, chat, format *recordedSpan
	for i := range exp.spans {
		s := &exp.spans[i]
		switch {
		case strings.HasPrefix(s.name, "invoke_agent"):
			invoke = s
		case strings.HasPrefix(s.name, "chat"):
			chat = s
		case strings.HasPrefix(s.name, "format"):
			format = s
		}
	}
	if invoke == nil {
		t.Fatalf("invoke_agent span not found, got: %v", spanNames(exp.spans))
	}
	if chat == nil {
		t.Fatalf("chat span not found, got: %v", spanNames(exp.spans))
	}
	if format == nil {
		t.Fatalf("format span not found, got: %v", spanNames(exp.spans))
	}

	requireGenAIMessages(t, invoke, "invoke_agent")
	requireGenAIMessages(t, chat, "chat")
	requireGenAIMessages(t, format, "format")
}

func spanNames(spans []recordedSpan) []string {
	out := make([]string, 0, len(spans))
	for _, s := range spans {
		out = append(out, s.name)
	}
	return out
}

func requireGenAIMessages(t *testing.T, s *recordedSpan, label string) {
	t.Helper()
	in, ok := s.attrs["gen_ai.input.messages"]
	if !ok {
		t.Fatalf("%s span missing gen_ai.input.messages (have: %v)", label, attrKeys(s))
	}
	if !json.Valid([]byte(in.AsString())) {
		t.Fatalf("%s span gen_ai.input.messages is not valid JSON: %q", label, in.AsString())
	}
	out, ok := s.attrs["gen_ai.output.messages"]
	if !ok {
		t.Fatalf("%s span missing gen_ai.output.messages (have: %v)", label, attrKeys(s))
	}
	if !json.Valid([]byte(out.AsString())) {
		t.Fatalf("%s span gen_ai.output.messages is not valid JSON: %q", label, out.AsString())
	}
}

func attrKeys(s *recordedSpan) []string {
	out := make([]string, 0, len(s.attrs))
	for k := range s.attrs {
		out = append(out, k)
	}
	return out
}
