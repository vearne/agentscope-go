package tracing

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	defaultServiceName  = "agentscope-go"
	instrumentationName = "github.com/vearne/agentscope-go"
)

var (
	globalMu sync.Mutex
	provider *sdktrace.TracerProvider
)

type TracingOption func(*tracingConfig)

type tracingConfig struct {
	serviceName string
	insecure    bool
	httpURLPath string
}

func WithServiceName(name string) TracingOption {
	return func(c *tracingConfig) { c.serviceName = name }
}

func WithInsecure() TracingOption {
	return func(c *tracingConfig) { c.insecure = true }
}

func WithHTTPURLPath(path string) TracingOption {
	return func(c *tracingConfig) { c.httpURLPath = path }
}

// SetupTracing initializes the global TracerProvider with an OTLP gRPC exporter.
// It returns a shutdown function that the caller should defer.
func SetupTracing(ctx context.Context, endpoint string, opts ...TracingOption) (func(context.Context) error, error) {
	globalMu.Lock()
	defer globalMu.Unlock()

	if provider != nil {
		return func(context.Context) error { return nil }, nil
	}

	cfg := tracingConfig{
		serviceName: defaultServiceName,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	exporterOpts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
	}
	if cfg.insecure {
		exporterOpts = append(exporterOpts, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptracegrpc.New(ctx, exporterOpts...)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	provider = tp

	return func(ctx context.Context) error {
		globalMu.Lock()
		defer globalMu.Unlock()
		if provider != nil {
			err := provider.Shutdown(ctx)
			provider = nil
			return err
		}
		return nil
	}, nil
}

// SetupTracingHTTP initializes the global TracerProvider with an OTLP HTTP exporter.
// It returns a shutdown function that the caller should defer.
func SetupTracingHTTP(ctx context.Context, endpoint string, opts ...TracingOption) (func(context.Context) error, error) {
	globalMu.Lock()
	defer globalMu.Unlock()

	if provider != nil {
		return func(context.Context) error { return nil }, nil
	}

	cfg := tracingConfig{
		serviceName: defaultServiceName,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	exporterOpts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpoint),
	}
	if cfg.httpURLPath != "" {
		exporterOpts = append(exporterOpts, otlptracehttp.WithURLPath(cfg.httpURLPath))
	}
	if cfg.insecure {
		exporterOpts = append(exporterOpts, otlptracehttp.WithInsecure())
	}

	exporter, err := otlptracehttp.New(ctx, exporterOpts...)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	// Use a synchronous span processor for OTLP HTTP (e.g. agentscope-studio):
	// short-lived CLIs often exit right after the last span ends; a batch
	// processor may not flush before process exit, and Studio would show spans
	// without the attributes set on End().
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	provider = tp

	return func(ctx context.Context) error {
		globalMu.Lock()
		defer globalMu.Unlock()
		if provider != nil {
			err := provider.Shutdown(ctx)
			provider = nil
			return err
		}
		return nil
	}, nil
}

// StartSpan creates a new span using the library's tracer.
func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	tracer := otel.Tracer(instrumentationName)
	opts := []trace.SpanStartOption{}
	if len(attrs) > 0 {
		opts = append(opts, trace.WithAttributes(attrs...))
	}
	return tracer.Start(ctx, name, opts...)
}

// SpanFromContext returns the current span from the context.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// Tracer returns a named tracer from the global provider.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
