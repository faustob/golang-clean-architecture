package middleware

import (
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// meter is the single package-level meter for this service; every instrument below is created
// from it.
var meter = otel.Meter("golang-clean-architecture")

// tracer is used to guarantee a RECORDING span for this middleware's own error-type/slow-request
// annotations, rather than assuming how/where an upstream framework middleware (otelfiber) stores
// its span in the Fiber context — that storage mechanism is internal and version-specific.
var tracer = otel.Tracer("golang-clean-architecture")

var (
	// httpServerRequestDuration is the OTel semantic-convention HTTP server latency histogram,
	// tagged with a tenant tier business dimension for cohort-aware P95/P99 SLOs.
	httpServerRequestDuration metric.Float64Histogram

	// httpServerRequestOutcomes is the request outcome counter: labeled by route and outcome
	// class (success|error), so availability is computable without scanning traces.
	httpServerRequestOutcomes metric.Int64Counter

	// httpServerRequestsByTenant is the per-tenant throughput counter, labeled by tenant and
	// route, so a single noisy tenant doesn't mask a capacity regression for everyone else.
	httpServerRequestsByTenant metric.Int64Counter
)

func init() {
	var err error

	httpServerRequestDuration, err = meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of inbound HTTP requests, in seconds"),
	)
	if err != nil {
		log.Printf("otel: failed to create http.server.request.duration histogram: %v", err)
	}

	httpServerRequestOutcomes, err = meter.Int64Counter(
		"http.server.request.outcomes",
		metric.WithUnit("1"),
		metric.WithDescription("Count of inbound HTTP requests by route and outcome class"),
	)
	if err != nil {
		log.Printf("otel: failed to create http.server.request.outcomes counter: %v", err)
	}

	httpServerRequestsByTenant, err = meter.Int64Counter(
		"http.server.requests.by_tenant",
		metric.WithUnit("1"),
		metric.WithDescription("Count of inbound HTTP requests by tenant, for per-tenant throughput SLOs"),
	)
	if err != nil {
		log.Printf("otel: failed to create http.server.requests.by_tenant counter: %v", err)
	}
}

// slowRequestThreshold gates the slow-request span event used to triage P99 tail-latency breaches.
const slowRequestThreshold = 1 * time.Second

// NewOtelMiddleware records the http.server.request.duration histogram and the request
// outcome/per-tenant counters for every request, and annotates a span it owns directly (started
// from this middleware's own tracer, nesting under any parent span already in the context) with
// error-type and slow-request info — so the annotation never depends on how an upstream
// framework middleware stores its span.
func NewOtelMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Start (and own) a span for this request so error/slow-request annotations always
		// land on a real, recording span — nesting under otelfiber's span when present, and
		// still correct if it is not. tracer.Start returns a NEW context carrying the span;
		// propagate it via SetUserContext so downstream handlers (and c.Next() below) nest
		// under it too.
		reqCtx, span := tracer.Start(c.UserContext(), "http.request")
		c.SetUserContext(reqCtx)
		defer span.End()

		err := c.Next()

		duration := time.Since(start)
		status := c.Response().StatusCode()

		// c.Route() is read AFTER next() has run, so Fiber has already resolved the matched
		// route and Path() reflects the registered PATTERN (e.g. /api/contacts/:contactId),
		// not the raw request path — keeping the http.route attribute low-cardinality.
		route := c.Route().Path
		if route == "" {
			route = "unknown"
		}

		tenantTier := c.Get("X-Tenant-Tier", "unknown")
		tenantID := c.Get("X-Tenant-ID", "unknown")

		outcome := "success"
		if status >= 500 {
			outcome = "error"
		}

		errType := ""
		if err != nil {
			errType = fmt.Sprintf("%T", err)
		} else if status >= 500 {
			errType = "http_5xx"
		}

		durationAttrs := []attribute.KeyValue{
			attribute.String("http.request.method", c.Method()),
			attribute.String("url.scheme", c.Protocol()),
			attribute.Int("http.response.status_code", status),
			attribute.String("http.route", route),
			attribute.String("tenant.tier", tenantTier),
		}
		if errType != "" {
			durationAttrs = append(durationAttrs, attribute.String("error.type", errType))
		}

		if httpServerRequestDuration != nil {
			httpServerRequestDuration.Record(reqCtx, duration.Seconds(), metric.WithAttributes(durationAttrs...))
		}

		if httpServerRequestOutcomes != nil {
			httpServerRequestOutcomes.Add(reqCtx, 1, metric.WithAttributes(
				attribute.String("http.route", route),
				attribute.String("outcome", outcome),
			))
		}

		if httpServerRequestsByTenant != nil {
			httpServerRequestsByTenant.Add(reqCtx, 1, metric.WithAttributes(
				attribute.String("tenant.id", tenantID),
				attribute.String("http.route", route),
			))
		}

		// span is OUR OWN span from tracer.Start above — guaranteed to be recording (bound to
		// the global TracerProvider registered in config.NewTelemetry), regardless of how any
		// upstream framework middleware manages its own span.
		if span.IsRecording() {
			if errType != "" {
				span.SetAttributes(attribute.String("error.type", errType))
				span.SetStatus(codes.Error, errType)
			}
			if duration > slowRequestThreshold {
				span.AddEvent("slow.request", trace.WithAttributes(
					attribute.Float64("duration.seconds", duration.Seconds()),
					attribute.String("http.route", route),
				))
			}
		}

		return err
	}
}
