package main

import (
	"fmt"
	"log"
	"net/http"
)

type apiConfig struct {
	fileserverHits int
}

func main() {
	apiCfg := &apiConfig{0}
	mux := http.NewServeMux()

	rootHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	assetsHandler := http.StripPrefix("/app/assets", http.FileServer(http.Dir("./assets")))

	mux.Handle("/app/", apiCfg.middlewareMetricsInc(rootHandler))
	mux.Handle("/app/assets", apiCfg.middlewareMetricsInc(assetsHandler))
	mux.HandleFunc("/healthz", healthCheck)
	mux.HandleFunc("/metrics", apiCfg.fileserverHitsHandler)
	mux.HandleFunc("/reset", apiCfg.fileserverHitsResetHandler)

	corsMux := middlewareCors(mux)
	server := http.Server{
		Addr:    ":8080",
		Handler: corsMux,
	}
	err := server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Request: %s %s", r.Method, r.URL.Path)
		cfg.fileserverHits++
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) getFileserverHits() string {
	return fmt.Sprintf("Hits: %d", cfg.fileserverHits)
}

func (cfg *apiConfig) fileserverHitsReset() {
	cfg.fileserverHits = 0
}

func (cfg *apiConfig) fileserverHitsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", " text/plain; charset=utf-8 ")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(cfg.getFileserverHits()))
}

func (cfg *apiConfig) fileserverHitsResetHandler(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHitsReset()
	w.Header().Set("Content-Type", " text/plain; charset=utf-8 ")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func middlewareCors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func healthCheck(w http.ResponseWriter, request *http.Request) {
	w.Header().Set("Content-Type", " text/plain; charset=utf-8 ")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
