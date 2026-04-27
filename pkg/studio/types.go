package studio

// RunData represents the payload sent to /trpc/registerRun.
// It matches the Python agentscope registerRun payload format.
type RunData struct {
	ID        string `json:"id"`
	Project   string `json:"project"`
	Name      string `json:"name"`
	Timestamp string `json:"timestamp"`
	PID       int    `json:"pid"`
	Status    string `json:"status"`
	RunDir    string `json:"run_dir"`
}

// PushMessageRequest represents the payload sent to /trpc/pushMessage.
// It matches the Python agentscope pushMessage payload format.
type PushMessageRequest struct {
	RunID     string                 `json:"runId"`
	ReplyID   string                 `json:"replyId"`
	ReplyName string                 `json:"replyName"`
	ReplyRole string                 `json:"replyRole"`
	Msg       map[string]interface{} `json:"msg"`
}
