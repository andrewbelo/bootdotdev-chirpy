package routers

import (
	"net/http"
	"os"

	"github.com/andrewbelo/bootdotdev-chirpy/internal/db"
	"github.com/go-chi/chi/v5"
)

type ApiConfig struct {
	FileserverHits int
	DB             *db.DB
	JWTSecret      string
	PolkaApiKey    string
}

func NewApiConfig(debug bool) (ApiConfig, error) {
	var cfg ApiConfig
	db_path := "chirps.json"
	if debug {
		db_path = "chirps_debug.json"
	}
	db, err := db.NewDB(db_path, debug)
	if err != nil {
		return cfg, err
	}
	cfg.DB = db
	cfg.JWTSecret = os.Getenv("JWT_SECRET")
	cfg.PolkaApiKey = os.Getenv("POLKA_API_KEY")
	return cfg, nil
}

func FinalRouter(apiCfg *ApiConfig) *chi.Mux {
	r := chi.NewRouter()
	r.Mount("/api", apiRoutes(apiCfg))
	r.Mount("/admin", adminRoutes(apiCfg))
	r.Mount("/app", appRoutes(apiCfg))
	return r
}

func appRoutes(cfg *ApiConfig) *chi.Mux {
	r := chi.NewRouter()
	rootHandler := cfg.middlewareMetricsInc(http.StripPrefix(
		"/app", http.FileServer(http.Dir(".")),
	))
	r.Handle("/app/*", rootHandler)
	r.Handle("/app", rootHandler)
	return r
}
