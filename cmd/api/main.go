package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/turman17/orbint/internal/store"
	"github.com/turman17/orbint/internal/orbit"
	"github.com/turman17/orbint/internal/celestrak"
	"github.com/turman17/orbint/internal/util"
)

func logging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }
        log.Printf("%s %s", r.Method, r.URL.Path)
        next.ServeHTTP(w, r)
    })
}

func satellites(db *store.Store) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        tles, err := db.ListSatellites()
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(tles)
    }
}

func history(db *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid satellite id", http.StatusBadRequest)
			return
		}

		// Default: last 30 days
		to := time.Now()
		from := to.AddDate(0, 0, -30)

		// Optional ?from=...&to=...
		if value := r.URL.Query().Get("from"); value != "" {
			from, err = time.Parse(time.RFC3339, value)
			if err != nil {
				http.Error(w, "invalid 'from' date", http.StatusBadRequest)
				return
			}
		}

		if value := r.URL.Query().Get("to"); value != "" {
			to, err = time.Parse(time.RFC3339, value)
			if err != nil {
				http.Error(w, "invalid 'to' date", http.StatusBadRequest)
				return
			}
		}

		if from.After(to) {
			http.Error(w, "'from' must be before 'to'", http.StatusBadRequest)
			return
		}

		data, err := db.GetHistory(id, from, to)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("failed to encode history: %v", err)
		}
	}
}

func position(client *celestrak.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid satellite id", http.StatusBadRequest)
			return
		}

		rawTLE, err := client.FetchTLE(id)
		if err != nil {
			http.Error(w, "failed to fetch satellite TLE", http.StatusBadGateway)
			return
		}

		p, err := orbit.NewPropagator(rawTLE)
		if err != nil {
			http.Error(w, "failed to create propagator", http.StatusInternalServerError)
			return
		}

		lat, lon, alt, err := p.Position(time.Now())
		if err != nil {
			http.Error(w, "failed to calculate position", http.StatusInternalServerError)
			return
		}

		response := struct {
			Lat float64 `json:"lat"`
			Lon float64 `json:"lon"`
			Alt float64 `json:"alt"`
		}{lat, lon, alt}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("failed to encode position: %v", err)
		}
	}
}

func main(){
	err := godotenv.Load()
	util.Check(err)
	client := celestrak.NewClient()

	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL - not found")
	}
	
	db, err := store.NewStore(connStr)
	util.Check(err)
	log.Println("API server starting on :8090")
	http.HandleFunc("GET /api/satellites", satellites(db))
	http.HandleFunc("GET /api/satellites/{id}/history", history(db))
	http.HandleFunc("GET /api/satellites/{id}/position", position(client))

	log.Fatal(http.ListenAndServe(":8090", logging(http.DefaultServeMux)))
}