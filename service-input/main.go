package main

import (
	"encoding/json"
	"net/http"
	"regexp"
)

type CepRequest struct {
	Cep string `json:"cep"`
}

func validateCep(cep string) bool {
	match, _ := regexp.MatchString(`^\d{8}$`, cep)
	return match
}

func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CepRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if !validateCep(req.Cep) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "invalid zipcode"})
		return
	}

	// Forward to Service B (replace URL with actual service B endpoint)
	serviceBUrl := "http://service-b/endpoint"
	resp, err := http.Post(serviceBUrl, "application/json", r.Body)
	if err != nil {
		http.Error(w, "failed to forward request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	_, _ = w.Write([]byte{})
}

func main() {
	http.HandleFunc("/cep", handler)
	http.ListenAndServe(":8080", nil)
}
