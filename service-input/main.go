package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"time"

	"github.com/kassiobuck/go-expert-fullcycle-otel/otel/otel_provider"
	appServer "github.com/kassiobuck/go-expert-fullcycle-otel/server"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

var serviceAName = "service_a"
var serviceBUrl = "http://service_b:8081"

type CityTemp struct {
	City  string  `json:"city"`
	TempC float64 `json:"tempC"`
	TempF float64 `json:"tempF"`
	TempK float64 `json:"tempK"`
}

func main() {

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	shutdown, err := otel_provider.InitProvider(serviceAName)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Fatal("failed to shutdown TracerProvider: %w", err)
		}
	}()

	sr := appServer.NewServer(serviceAName, otel.Tracer(serviceAName))
	routes := sr.CreateServer([]appServer.Route{
		{Path: "/cep", Handler: cepPostHandler},
	})

	http.ListenAndServe(":8080", routes)

	select {
	case <-sigCh:
		log.Println("Shutting down gracefully, CTRL+C pressed...")
	case <-ctx.Done():
		log.Println("Shutting down due to other reason...")
	}

	// Create a timeout context for the graceful shutdown
	_, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
}

func cepPostHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cep := r.FormValue("cep")
	cep = strings.ReplaceAll(cep, "[^0-9]", "")
	if cep == "" || len(cep) != 8 {
		http.Error(w, "Invalid zipcode", http.StatusBadRequest)
		return
	}

	//delay to simulate processing time
	time.Sleep(50 * time.Millisecond)

	cityTemp := orchestrator(ctx, cep, w)
	if cityTemp == nil {
		http.Error(w, "Failed to get city temperature", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cityTemp)
}

func orchestrator(ctx context.Context, cep string, w http.ResponseWriter) *CityTemp {
	req, err := http.NewRequestWithContext(ctx, "GET", serviceBUrl+"/clima?cep="+cep, nil)
	propagator := otel.GetTextMapPropagator() // Get the global propagator
	propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))

	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return nil
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to call service B", http.StatusInternalServerError)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, resp.Status, resp.StatusCode)
		return nil
	}

	var cityTemp CityTemp

	if err := json.NewDecoder(resp.Body).Decode(&cityTemp); err != nil {
		http.Error(w, "Failed to decode response from service B", http.StatusInternalServerError)
		return nil
	}
	return &cityTemp
}
