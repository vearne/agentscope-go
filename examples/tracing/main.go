// This example demonstrates OpenTelemetry tracing integration.
// It shows how to set up tracing and create spans around agent operations.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/vearne/agentscope-go/pkg/agent"
	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/message"
	"github.com/vearne/agentscope-go/pkg/model"
	"github.com/vearne/agentscope-go/pkg/tracing"
	"go.opentelemetry.io/otel/attribute"
)

func main() {
	shutdown, err := tracing.SetupTracing(
		context.Background(),
		"localhost:4317",
		tracing.WithServiceName("agentscope-example"),
		tracing.WithInsecure(),
	)
	if err != nil {
		log.Fatalf("setup tracing: %v", err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			log.Printf("shutdown tracer: %v", err)
		}
	}()

	ctx, span := tracing.StartSpan(
		context.Background(),
		"example.run",
		attribute.String("example.name", "tracing"),
	)
	defer span.End()

	m := model.NewOpenAIChatModel("gpt-4o", "your-api-key", "", false)
	f := formatter.NewOpenAIChatFormatter()

	a := agent.NewReActAgent(
		agent.WithReActName("traced-agent"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
		agent.WithReActPreReply(func(ctx context.Context, ag agent.AgentBase, msg *message.Msg, _ *message.Msg) {
			_, s := tracing.StartSpan(ctx, "agent.pre_reply",
				attribute.String("agent.name", ag.Name()),
			)
			s.End()
		}),
		agent.WithReActPostReply(func(ctx context.Context, ag agent.AgentBase, msg *message.Msg, resp *message.Msg) {
			_, s := tracing.StartSpan(ctx, "agent.post_reply",
				attribute.String("agent.name", ag.Name()),
				attribute.Int("response.length", len(resp.GetTextContent())),
			)
			s.End()
		}),
	)

	replyCtx, replySpan := tracing.StartSpan(ctx, "agent.reply",
		attribute.String("agent.name", a.Name()),
	)
	msg := agent.NewUserMsg("user", "Tell me a fun fact about Go.")
	resp, err := a.Reply(replyCtx, msg)
	replySpan.End()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("[%s] %s\n", resp.Name, resp.GetTextContent())
}
