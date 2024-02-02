package routers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/andrewbelo/bootdotdev-chirpy/internal/db"
	"github.com/go-chi/chi/v5"
)

type chirpRequest struct {
	Body string `json:"body"`
}
type chirpResponse struct {
	AuthorID int    `json:"author_id"`
	Body     string `json:"body"`
	ID       int    `json:"id"`
}

func apiRoutes(cfg *ApiConfig) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/healthz", healthCheck)
	r.HandleFunc("/reset", cfg.fileserverHitsResetHandler)

	r.Get("/chirps", cfg.getChirpsHandler)
	r.Post("/chirps", cfg.createChirpHandler)
	r.Get("/chirps/{id}", cfg.getSingleChirpHandler)
	r.Delete("/chirps/{id}", cfg.deleteChirpHandler)

	r.Post("/users", cfg.createUserHandler)
	r.Post("/login", cfg.loginHandler)
	r.Put("/users", cfg.updateUserHandler)
	r.Post("/refresh", cfg.refreshTokenHandler)
	r.Post("/revoke", cfg.revokeTokenHandler)

	r.Post("/polka/webhooks", cfg.polkaWebhook)

	return r
}

func adminRoutes(cfg *ApiConfig) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/metrics", cfg.fileserverHitsHandler)
	return r
}

func validateChirp(chirp string) (string, error) {
	if len(chirp) > 140 {
		return "", fmt.Errorf("Chirp is too long")
	}
	swearWords := []string{
		"kerfuffle", "sharbert", "fornax",
	}
	re := regexp.MustCompile(fmt.Sprintf(`(?i)\b(%s)\b`,
		strings.Join(swearWords, "|"),
	))
	chirp = re.ReplaceAllString(chirp, "****")
	return chirp, nil
}

func (cfg *ApiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Request: %s %s", r.Method, r.URL.Path)
		cfg.FileserverHits++
		next.ServeHTTP(w, r)
	})
}

func (cfg *ApiConfig) getFileserverHits() string {
	html_template := `
	<html> <body>
		<h1>Welcome, Chirpy Admin</h1> <p>Chirpy has been visited %d times!</p>
	</body> </html>`
	return fmt.Sprintf(html_template, cfg.FileserverHits)
}

func (cfg *ApiConfig) fileserverHitsReset() {
	cfg.FileserverHits = 0
}

func (cfg *ApiConfig) fileserverHitsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(cfg.getFileserverHits()))
}

func (cfg *ApiConfig) fileserverHitsResetHandler(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHitsReset()
	w.Header().Set("Content-Type", " text/plain; charset=utf-8 ")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

type ChirpOut struct {
	AuthorId int    `json:"author_id"`
	Body     string `json:"body"`
	ID       int    `json:"id"`
}

func (cfg *ApiConfig) getChirpsHandler(w http.ResponseWriter, r *http.Request) {
	var chirpsOut []ChirpOut
	var chirps []db.Chirp
	var err error

	queryParams := r.URL.Query()
	sort := "asc"
	userID := -1
	filterByUser := false

	if queryParams.Has("sort") {
		sort = queryParams.Get("sort")
	}
	if queryParams.Has("user_id") {
		userID, err = strconv.Atoi(queryParams.Get("user_id"))
		filterByUser = true
	}
	if err != nil {
		marshalError(w, err, http.StatusBadRequest)
		return
	}

	if filterByUser {
		chirps, err = cfg.DB.GetChirpsByUser(userID, sort)
	} else {
		chirps, err = cfg.DB.GetChirpsOrderedBy(sort)
	}
	if err != nil {
		marshalError(w, err, http.StatusInternalServerError)
		return
	}
	for _, chirp := range chirps {
		chirpsOut = append(chirpsOut, ChirpOut{
			AuthorId: chirp.AuthorID, Body: chirp.Body, ID: chirp.ID})
	}
	marshallOK(w, chirpsOut)
}

func (cfg *ApiConfig) getSingleChirpHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		marshalError(w, err, http.StatusBadRequest)
		return
	}
	chirp, err := cfg.DB.GetChirp(id)
	if err != nil {
		marshalError(w, err, http.StatusNotFound)
		return
	}
	marshallOK(w, chirp)
}

func (cfg *ApiConfig) createChirpHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := cfg.checkAccessJWTInHeader(r.Header.Get("Authorization"))
	if err != nil {
		marshalError(w, err, http.StatusUnauthorized)
		return
	}
	var chirpR chirpRequest
	err = json.NewDecoder(r.Body).Decode(&chirpR)
	if err != nil {
		marshalError(w, err, http.StatusInternalServerError)
		return
	}
	body, error := validateChirp(chirpR.Body)
	if error != nil {
		marshalError(w, error, http.StatusBadRequest)
		return
	}

	chirp, err := cfg.DB.CreateChirp(body, userID)
	if err != nil {
		marshalError(w, err, http.StatusInternalServerError)
		return
	}
	marshallCreated(w, chirpResponse{chirp.AuthorID, chirp.Body, chirp.ID})
}

func (cfg *ApiConfig) deleteChirpHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := cfg.checkAccessJWTInHeader(r.Header.Get("Authorization"))
	if err != nil {
		marshalError(w, err, http.StatusUnauthorized)
		return
	}
	chirpID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		marshalError(w, err, http.StatusNotFound)
		return
	}
	err = cfg.DB.DeleteChirp(chirpID, userID)
	if err != nil {
		marshalError(w, err, http.StatusForbidden)
		return
	}
	marshallEmptyOK(w)
}

func healthCheck(w http.ResponseWriter, request *http.Request) {
	w.Header().Set("Content-Type", " text/plain; charset=utf-8 ")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
