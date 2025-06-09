package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"regexp"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// Response structure
type TempResponse struct {
	City  string  `json:"city"`
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

// ViaCEP response structure
type ViaCEPResponse struct {
	Localidade string `json:"localidade"`
	Erro       bool   `json:"erro,omitempty"`
}

// Validate CEP: must be 8 digits
func validateCEP(cep string) bool {
	re := regexp.MustCompile(`^\d{8}$`)
	return re.MatchString(cep)
}

// Service A: Busca CEP
func buscaCEP(ctx context.Context, tracer trace.Tracer, cep string) (string, error) {
	ctx, span := tracer.Start(ctx, "buscaCEP")
	defer span.End()

	url := fmt.Sprintf("https://viacep.com.br/ws/%s/json/", cep)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var data ViaCEPResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}
	if data.Erro {
		return "", errors.New("not found")
	}
	return data.Localidade, nil
}

// Service B: Busca Temperatura
func buscaTemperatura(ctx context.Context, tracer trace.Tracer, city string) (float64, error) {
	ctx, span := tracer.Start(ctx, "buscaTemperatura")
	defer span.End()

	apiKey := os.Getenv("WEATHERAPI_KEY")
	if apiKey == "" {
		return 0, errors.New("WEATHERAPI_KEY not set")
	}
	url := fmt.Sprintf("https://api.weatherapi.com/v1/current.json?key=%s&q=%s", apiKey, city)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Current struct {
			TempC float64 `json:"temp_c"`
		} `json:"current"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	return result.Current.TempC, nil
}

// Convert Celsius to Fahrenheit
func celsiusToFahrenheit(celsius float64) float64 {
	f := celsius*1.8 + 32
	return math.Round(f*10) / 10
}

// Convert Celsius to Kelvin
func celsiusToKelvin(celsius float64) float64 {
	k := celsius + 273.15
	return math.Round(k*10) / 10
}

// OTEL setup
func initTracer() (func(context.Context) error, error) {
	zipkinURL := "http://localhost:9411/api/v2/spans"
	exporter, err := zipkin.New(zipkinURL)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("service-orchestrator"),
		)),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

func main() {
	shutdown, err := initTracer()
	if err != nil {
		log.Fatalf("failed to initialize tracer: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		_ = shutdown(ctx)
	}()

	tracer := otel.Tracer("service-orchestrator")

	http.HandleFunc("/temperature", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "request")
		defer span.End()

		cep := r.URL.Query().Get("cep")
		if !validateCEP(cep) {
			http.Error(w, "invalid zipcode", http.StatusUnprocessableEntity)
			return
		}

		city, err := buscaCEP(ctx, tracer, cep)
		if err != nil {
			http.Error(w, "can not find zipcode", http.StatusNotFound)
			return
		}

		tempC, err := buscaTemperatura(ctx, tracer, city)
		if err != nil {
			http.Error(w, "can not find temperature", http.StatusInternalServerError)
			return
		}

		tempF := celsiusToFahrenheit(tempC)
		tempK := celsiusToKelvin(tempC)

		resp := TempResponse{
			City:  city,
			TempC: tempC,
			TempF: tempF,
			TempK: tempK,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
