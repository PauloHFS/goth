package tracing

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer trace.Tracer
	tp     *sdktrace.TracerProvider
)

type Config struct {
	ServiceName string
	Endpoint    string
	UseTLS      bool
	Protocol    string // "grpc" or "http"
	SampleRate  float64
	UseStdout   bool // For development - print to console
}

func Init(cfg Config) (func(context.Context) error, error) {
	ctx := context.Background()

	var exporter sdktrace.SpanExporter
	var err error

	if cfg.UseStdout {
		exporter, err = stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
			stdouttrace.WithWriter(os.Stdout),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
		}
	} else {
		endpoint := cfg.Endpoint
		if endpoint == "" {
			endpoint = "localhost:4317"
		}

		if cfg.Protocol == "http" {
			exporter, err = otlptracehttp.New(ctx,
				otlptracehttp.WithEndpoint(endpoint),
				otlptracehttp.WithInsecure(),
			)
		} else {
			exporter, err = otlptracegrpc.New(ctx,
				otlptracegrpc.WithEndpoint(endpoint),
				otlptracegrpc.WithInsecure(),
			)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	serviceName := cfg.ServiceName
	if serviceName == "" {
		serviceName = "goth"
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.DeploymentEnvironmentKey.String(env),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	sampler := sdktrace.AlwaysSample()
	if cfg.SampleRate > 0 && cfg.SampleRate < 1.0 {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	tp = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	otel.SetTracerProvider(tp)
	tracer = tp.Tracer(serviceName)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

func InitNoop() {
	tracer = otel.Tracer("noop")
}

func Tracer() trace.Tracer {
	if tracer == nil {
		InitNoop()
	}
	return tracer
}

// StartSpan inicia um novo span com contexto
func StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name)
}

// StartSpanWithOptions inicia um span com opções adicionais
func StartSpanWithOptions(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name, opts...)
}

// EndSpan finaliza um span com tratamento de erro
func EndSpan(span trace.Span, err error) {
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}
	span.End()
}

// AddSpanAttributes adiciona atributos ao span atual
func AddSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(attrs...)
	}
}

// RecordError registra um erro no span atual
func RecordError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

// RecordErrorWithAttributes registra erro com atributos adicionais
func RecordErrorWithAttributes(ctx context.Context, err error, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attrs...)
	}
}

// SetSpanStatus define o status do span
func SetSpanStatus(ctx context.Context, statusCode codes.Code, description string) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetStatus(statusCode, description)
	}
}

// ExtractContext extrai contexto de tracing de headers HTTP
func ExtractContext(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

// InjectContext injeta contexto de tracing em headers HTTP
func InjectContext(ctx context.Context, carrier propagation.TextMapCarrier) {
	otel.GetTextMapPropagator().Inject(ctx, carrier)
}

// HTTPCarrier é um carrier simples para headers HTTP
type HTTPCarrier map[string]string

var _ propagation.TextMapCarrier = (*HTTPCarrier)(nil)

// Get retorna um valor do carrier
func (c HTTPCarrier) Get(key string) string {
	return c[key]
}

// Set define um valor no carrier
func (c HTTPCarrier) Set(key string, value string) {
	c[key] = value
}

// Keys retorna todas as chaves do carrier
func (c HTTPCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

type SpanFunc func(ctx context.Context) error

func WithSpan(ctx context.Context, name string, fn SpanFunc) error {
	ctx, span := StartSpan(ctx, name)
	defer span.End()

	start := time.Now()
	err := fn(ctx)
	elapsed := time.Since(start)

	span.SetAttributes(
		attribute.Int64("duration_ms", elapsed.Milliseconds()),
	)

	if err != nil {
		RecordError(ctx, err)
		return err
	}

	return nil
}

// WithSpanAndResult executa uma função com span e retorna resultado
func WithSpanAndResult[T any](ctx context.Context, name string, fn func(ctx context.Context) (T, error)) (T, error) {
	ctx, span := StartSpan(ctx, name)
	defer span.End()

	start := time.Now()
	result, err := fn(ctx)
	elapsed := time.Since(start)

	span.SetAttributes(
		attribute.Int64("duration_ms", elapsed.Milliseconds()),
	)

	if err != nil {
		RecordError(ctx, err)
	}

	return result, err
}

func Shutdown(ctx context.Context) error {
	if tp != nil {
		return tp.Shutdown(ctx)
	}
	return nil
}
