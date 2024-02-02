package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/andrewbelo/bootdotdev-chirpy/internal/db"
	"github.com/andrewbelo/bootdotdev-chirpy/internal/routers"
	"github.com/joho/godotenv"
)

func main() {
	dbg := flag.Bool("debug", false, "Enable debug mode")
	godotenv.Load()
	flag.Parse()

	apiCfg, err := routers.NewApiConfig(*dbg)
	if err != nil {
		panic(err)
	}
	server := http.Server{
		Addr:    ":8080",
		Handler: routers.MiddlewareCors(routers.FinalRouter(&apiCfg)),
	}
	log.Println("Listening on :8080")
	err = server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}

func setupDB(dbg bool) *db.DB {
	db_path := "chirps.json"
	if dbg {
		log.Println("Debug mode enabled")
		db_path = "chirps_debug.json"
	}
	db, err := db.NewDB(db_path, dbg)
	if err != nil {
		panic(err)
	}
	return db
}
