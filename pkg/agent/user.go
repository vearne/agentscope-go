package agent

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/vearne/agentscope-go/internal/utils"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/message"
)

type UserAgent struct {
	id   string
	name string
	mem  memory.MemoryBase
}

func NewUserAgent(name string) *UserAgent {
	return &UserAgent{
		id:   utils.ShortUUID(),
		name: name,
		mem:  memory.NewInMemoryMemory(),
	}
}

func (a *UserAgent) ID() string   { return a.id }
func (a *UserAgent) Name() string { return a.name }

func (a *UserAgent) Reply(ctx context.Context, _ *message.Msg) (*message.Msg, error) {
	input := a.readInput()
	if input == "" {
		return nil, fmt.Errorf("empty input")
	}

	msg := message.NewMsg(a.name, input, "user")
	a.mem.Add(ctx, msg)
	return msg, nil
}

func (a *UserAgent) Observe(ctx context.Context, msg *message.Msg) error {
	if msg != nil {
		return a.mem.Add(ctx, msg)
	}
	return nil
}

func (a *UserAgent) Memory() memory.MemoryBase {
	return a.mem
}

func (a *UserAgent) readInput() string {
	fmt.Printf("[%s] > ", a.name)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	return line
}

func (a *UserAgent) Interrupt() {}

func (a *UserAgent) HandleInterrupt(_ context.Context, _ *message.Msg) (*message.Msg, error) {
	return nil, fmt.Errorf("UserAgent does not support interrupt handling")
}

func NewUserMsg(name, content string) *message.Msg {
	msg := message.NewMsg(name, content, "user")
	msg.Timestamp = time.Now().Format("2006-01-02 15:04:05.000")
	return msg
}
