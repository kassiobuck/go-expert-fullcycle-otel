package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"time"

	"github.com/kassiobuck/go-expert-fullcycle-otel/otel/otel_provider"
	appServer "github.com/kassiobuck/go-expert-fullcycle-otel/server"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
)

func init() {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}
	viper.SetConfigFile("./env/service-orchestrator." + env + ".env")
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

	ctx, cancel := signal.NotifyContext(context.Background(), os.Kill, os.Interrupt)
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
		{Path: "/clima", Handler: tempGetHandler},
	})

	http.ListenAndServe(":8081", routes)

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

func tempGetHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	weatherApiKey := viper.GetString("WEATHER_API_KEY")
	cep := r.URL.Query().Get("cep")
	if len(cep) != 8 {
		http.Error(w, "invalid zipcode", http.StatusUnprocessableEntity)
		return
	}

	viacepURL := fmt.Sprintf("https://viacep.com.br/ws/%s/json/", cep)
	resp, err := http.Get(viacepURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	var viacepResp struct {
		Localidade string `json:"localidade"`
		Erro       bool   `json:"erro"`
	}

	err = json.NewDecoder(resp.Body).Decode(&viacepResp)

	if err != nil || viacepResp.Erro {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	city := normalize(viacepResp.Localidade)

	weatherURL := fmt.Sprintf("http://api.weatherapi.com/v1/current.json?key=%s&q=%s", weatherApiKey, city)
	wresp, err := http.Get(weatherURL)
	if err != nil {
		http.Error(w, "error fetching weather", http.StatusInternalServerError)
		return
	}
	defer wresp.Body.Close()

	var weatherResp struct {
		Current struct {
			TempC float64 `json:"temp_c"`
		} `json:"current"`
	}
	if err := json.NewDecoder(wresp.Body).Decode(&weatherResp); err != nil {
		http.Error(w, "error parsing weather data", http.StatusInternalServerError)
		return
	}

	tempC := weatherResp.Current.TempC
	tempF := tempC*1.8 + 32
	tempK := tempC + 273

	response := struct {
		City  string  `json:"city"`
		TempC float64 `json:"tempC"`
		TempF float64 `json:"tempF"`
		TempK float64 `json:"tempK"`
	}{
		City:  viacepResp.Localidade,
		TempC: tempC,
		TempF: tempF,
		TempK: tempK,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Remove acentos e substitui ç por c
func normalize(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, "+")
	replacements := []struct {
		old string
		new string
	}{
		{"á", "a"}, {"à", "a"}, {"ã", "a"}, {"â", "a"}, {"ä", "a"},
		{"é", "e"}, {"è", "e"}, {"ê", "e"}, {"ë", "e"},
		{"í", "i"}, {"ì", "i"}, {"î", "i"}, {"ï", "i"},
		{"ó", "o"}, {"ò", "o"}, {"õ", "o"}, {"ô", "o"}, {"ö", "o"},
		{"ú", "u"}, {"ù", "u"}, {"û", "u"}, {"ü", "u"},
		{"ç", "c"},
	}
	for _, r := range replacements {
		s = strings.ReplaceAll(s, r.old, r.new)
	}
	return s
}
