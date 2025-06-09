package main

import (
	"context"
	"log"
	"net/http"

	zipkinhttp "github.com/openzipkin/zipkin-go/reporter/http"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/zipkinexporter"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
)

func main() {
	// Configure Zipkin
	reporter := zipkinhttp.NewReporter("http://localhost:9411/api/v2/spans")
	defer reporter.Close()

	// Configure OTEL collector
	cfg := config.New()

	// Create OTLP receiver
	factory := otlpreceiver.NewFactory()
	receiverConfig := factory.CreateDefaultConfig()
	receiver, err := factory.CreateTracesReceiver(context.Background(), component.ReceiverCreateSettings{}, receiverConfig, nil)
	if err != nil {
		log.Fatalf("Failed to create receiver: %v", err)
	}

	// Create Zipkin exporter
	zipkinFactory := zipkinexporter.NewFactory()
	exporterConfig := zipkinFactory.CreateDefaultConfig()
	exporter, err := zipkinFactory.CreateTracesExporter(context.Background(), component.ExporterCreateSettings{}, exporterConfig)
	if err != nil {
		log.Fatalf("Failed to create exporter: %v", err)
	}

	// Start the collector
	err = receiver.Start(context.Background(), nil)
	if err != nil {
		log.Fatalf("Failed to start receiver: %v", err)
	}

	// Start Zipkin server
	http.HandleFunc("/api/v2/spans", func(w http.ResponseWriter, r *http.Request) {
		// Handle incoming spans
		w.WriteHeader(http.StatusOK)
	})

	log.Println("Starting server on :9411")
	if err := http.ListenAndServe(":9411", nil); err != nil {
		log.Fatal(err)
	}

	defer func() {
		receiver.Shutdown(context.Background())
		exporter.Shutdown(context.Background())
	}()
}
