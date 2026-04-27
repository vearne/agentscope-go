package studio

import (
	"context"
	"log"

	"github.com/vearne/agentscope-go/pkg/message"
)

// ForwardMessage pushes a message to the studio server. Used by agent hooks
// to forward both user input and assistant response messages.
func ForwardMessage(ctx context.Context, agentName, replyRole string, msg *message.Msg) {
	client := GetClient()
	if client == nil || msg == nil {
		return
	}

	err := client.PushMessage(ctx, &PushMessageRequest{
		RunID:     client.RunID(),
		ReplyID:   msg.ID,
		ReplyName: agentName,
		ReplyRole: replyRole,
		Msg:       MsgToPayload(msg),
	})
	if err != nil {
		log.Printf("studio: pushMessage failed: %v", err)
	}
}
