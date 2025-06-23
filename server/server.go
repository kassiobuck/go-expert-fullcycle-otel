package appServer

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

type Server struct {
	Tracer     trace.Tracer
	ServerName string
}

type HandlerWithContext func(context.Context, http.ResponseWriter, *http.Request)

type Route struct {
	Path    string
	Handler HandlerWithContext
}

// NewServer creates a new server instance
func NewServer(serverName string, tracer trace.Tracer) *Server {
	return &Server{
		Tracer:     tracer,
		ServerName: serverName,
	}
}

// createServer creates a new server instance with go chi router
func (s *Server) CreateServer(routes []Route) *chi.Mux {
	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Logger)
	router.Use(middleware.Timeout(60 * time.Second))

	// promhttp prometheus metrics endpoint
	router.Handle("/metrics", promhttp.Handler())
	for _, route := range routes {
		router.HandleFunc(route.Path, s.handlerMidleware(route.Handler))
	}
	return router
}

func (s *Server) handlerMidleware(paramFunc HandlerWithContext) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		carrier := propagation.HeaderCarrier(r.Header)
		ctx := r.Context()
		ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
		otel.GetTextMapPropagator().Inject(ctx, carrier)

		spanOptions := []trace.SpanStartOption{
			trace.WithAttributes(semconv.HTTPMethodKey.String(r.Method)),
			trace.WithAttributes(semconv.HTTPURLKey.String(r.URL.String())),
			trace.WithAttributes(semconv.NetHostIPKey.String(r.Header.Get("x-forwarded-for"))),
		}

		ctx, span := s.Tracer.Start(ctx, "SPAN_"+s.ServerName, spanOptions...)
		//ctx, span := s.Tracer.Start(ctx, "SPAN_"+s.ServerName)
		span.SetAttributes(attribute.String("server.name", s.ServerName))
		span.AddEvent("Request received", trace.WithAttributes())

		defer span.End()
		paramFunc(ctx, w, r)
	}

}
