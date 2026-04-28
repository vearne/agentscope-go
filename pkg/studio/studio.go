package studio

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/vearne/agentscope-go/internal/utils"
	"github.com/vearne/agentscope-go/pkg/tracing"
)

var (
	globalMu              sync.Mutex
	globalClient          *StudioClient
	globalTracingShutdown func(context.Context) error
)

// Option configures the studio client.
type Option func(*config)

type config struct {
	url     string
	project string
	name    string
	runID   string
}

// WithURL sets the studio server URL (e.g. "http://localhost:3000").
func WithURL(url string) Option {
	return func(c *config) { c.url = url }
}

// WithProject sets the project name reported to studio.
func WithProject(project string) Option {
	return func(c *config) { c.project = project }
}

// WithName sets the run name reported to studio.
func WithName(name string) Option {
	return func(c *config) { c.name = name }
}

// WithRunID sets a custom run ID. If not provided, one is auto-generated.
func WithRunID(id string) Option {
	return func(c *config) { c.runID = id }
}

// Init initializes the studio connection, registers the run, and sets the
// global client. After calling Init, all subsequently created ReActAgent
// instances will automatically forward messages to the studio.
//
// Init is idempotent: calling it multiple times is a no-op after the first
// successful initialization.
func Init(opts ...Option) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalClient != nil {
		return nil
	}

	cfg := config{
		project: "UnnamedProject_At" + time.Now().Format("20060102"),
		name:    time.Now().Format("150405_") + utils.ShortUUID()[:4],
		runID:   utils.ShortUUID(),
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.url == "" {
		return fmt.Errorf("studio: URL is required")
	}

	client := newStudioClient(cfg.url, cfg.runID, cfg.project, cfg.name)

	if err := client.RegisterRun(context.Background()); err != nil {
		return fmt.Errorf("studio: register run: %w", err)
	}

	globalClient = client

	// OTLP HTTP exporter expects endpoint as host:port and URL path separately.
	parsedURL, err := url.Parse(cfg.url)
	if err != nil {
		log.Printf("studio: invalid URL %q, tracing disabled: %v", cfg.url, err)
		return nil
	}
	if parsedURL.Host == "" {
		log.Printf("studio: invalid URL %q (missing host), tracing disabled", cfg.url)
		return nil
	}
	shutdownTracing, err := tracing.SetupTracingHTTP(
		context.Background(),
		parsedURL.Host,
		tracing.WithInsecure(),
		tracing.WithHTTPURLPath("/v1/traces"),
	)
	if err != nil {
		log.Printf("studio: failed to setup tracing: %v", err)
	} else {
		globalTracingShutdown = shutdownTracing
	}

	return nil
}

// Shutdown disconnects from the studio, clearing the global client.
func Shutdown(ctx context.Context) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalTracingShutdown != nil {
		if err := globalTracingShutdown(ctx); err != nil {
			return err
		}
		globalTracingShutdown = nil
	}
	globalClient = nil
	return nil
}

// GetClient returns the global studio client, or nil if Init has not been called.
func GetClient() *StudioClient {
	globalMu.Lock()
	defer globalMu.Unlock()
	return globalClient
}
