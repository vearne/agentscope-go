package studio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	maxRetries     = 3
	retryDelay     = 100 * time.Millisecond
	requestTimeout = 5 * time.Second
)

// StudioClient is an HTTP client for the agentscope-studio tRPC API.
type StudioClient struct {
	baseURL    string
	runID      string
	project    string
	name       string
	httpClient *http.Client
}

// newStudioClient creates a new StudioClient. It does NOT call RegisterRun;
// the caller should invoke RegisterRun separately.
func newStudioClient(baseURL, runID, project, name string) *StudioClient {
	return &StudioClient{
		baseURL: baseURL,
		runID:   runID,
		project: project,
		name:    name,
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
	}
}

// RegisterRun POSTs to /trpc/registerRun to register this run with the studio.
func (c *StudioClient) RegisterRun(ctx context.Context) error {
	data := RunData{
		ID:        c.runID,
		Project:   c.project,
		Name:      c.name,
		Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
		PID:       os.Getpid(),
		Status:    "running",
		RunDir:    "",
	}

	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal run data: %w", err)
	}

	return c.postWithRetry(ctx, "/trpc/registerRun", body)
}

// PushMessage POSTs to /trpc/pushMessage to forward a message to the studio.
func (c *StudioClient) PushMessage(ctx context.Context, req *PushMessageRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal push message request: %w", err)
	}

	return c.postWithRetry(ctx, "/trpc/pushMessage", body)
}

// RunID returns the run ID associated with this client.
func (c *StudioClient) RunID() string {
	return c.runID
}

// postWithRetry sends a POST request with up to maxRetries retries on failure.
func (c *StudioClient) postWithRetry(ctx context.Context, path string, body []byte) error {
	url := c.baseURL + path
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryDelay):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("studio: %s attempt %d failed: %v", path, attempt+1, err)
			continue
		}
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		lastErr = fmt.Errorf("studio: %s returned status %d", path, resp.StatusCode)
		log.Printf("studio: %s attempt %d returned %d", path, attempt+1, resp.StatusCode)
	}

	return lastErr
}
