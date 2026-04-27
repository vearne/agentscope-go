package tracing

import (
	"context"
	"testing"
)

func TestSetupTracingHTTP_SetsAndShutsDownProvider(t *testing.T) {
	ctx := context.Background()

	shutdown, err := SetupTracingHTTP(ctx, "localhost:4318", WithInsecure())
	if err != nil {
		t.Fatalf("SetupTracingHTTP returned error: %v", err)
	}
	if shutdown == nil {
		t.Fatalf("SetupTracingHTTP returned nil shutdown")
	}

	// Calling again should be a no-op while provider is initialized.
	shutdown2, err := SetupTracingHTTP(ctx, "localhost:4318", WithInsecure())
	if err != nil {
		t.Fatalf("SetupTracingHTTP (second call) returned error: %v", err)
	}
	if shutdown2 == nil {
		t.Fatalf("SetupTracingHTTP (second call) returned nil shutdown")
	}
	if err := shutdown2(ctx); err != nil {
		t.Fatalf("shutdown2 returned error: %v", err)
	}

	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}
}
