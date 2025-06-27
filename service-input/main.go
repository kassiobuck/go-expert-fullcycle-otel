package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"

	"time"

	"github.com/kassiobuck/go-expert-fullcycle-otel/otel/otel_provider"
	appServer "github.com/kassiobuck/go-expert-fullcycle-otel/server"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type CityTemp struct {
	City  string  `json:"city"`
	TempC float64 `json:"tempC"`
	TempF float64 `json:"tempF"`
	TempK float64 `json:"tempK"`
}

func init() {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}
	viper.SetConfigFile("./env/service-input." + env + ".env")
	err := viper.ReadInConfig()
	if err != nil {
		log.Println("fatal error config file: %w", err)
	}
}

func main() {

	serviceName := viper.GetString("SERVICE_NAME")
	otelUrl := viper.GetString("OTEL_URL")
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	shutdown, err := otel_provider.InitProvider(serviceName, otelUrl)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Fatal("failed to shutdown TracerProvider: %w", err)
		}
	}()

	sr := appServer.NewServer(serviceName, otel.Tracer(serviceName))
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

	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	var data = struct {
		Cep string `json:"cep"`
	}{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	re := regexp.MustCompile(`[^0-9]`)
	cep := re.ReplaceAllString(data.Cep, "")

	if len(cep) != 8 {
		http.Error(w, "Invalid zipcode", http.StatusUnprocessableEntity)
		return
	}

	//delay to simulate processing time
	time.Sleep(50 * time.Millisecond)

	orchestratorRequest(ctx, cep, w)

}

func orchestratorRequest(ctx context.Context, cep string, w http.ResponseWriter) {
	serviceBUrl := viper.GetString("SERVICE_B_URL")
	req, err := http.NewRequestWithContext(ctx, "GET", serviceBUrl+"/clima?cep="+cep, nil)
	propagator := otel.GetTextMapPropagator() // Get the global propagator
	propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))

	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to call service B", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, resp.Status, resp.StatusCode)
		return
	}

	var cityTemp CityTemp

	if err := json.NewDecoder(resp.Body).Decode(&cityTemp); err != nil {
		http.Error(w, "Failed to decode response from service B", http.StatusInternalServerError)
		return
	}

	if cityTemp.City == "" {
		http.Error(w, "Failed to get city temperature", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cityTemp)
}
