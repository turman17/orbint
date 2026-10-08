package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/turman17/orbint/internal/detect"
	"github.com/turman17/orbint/internal/feature"
	"github.com/turman17/orbint/internal/store"
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

// queryRange reads the optional RFC 3339 ?from= and ?to= parameters,
// defaulting to the trailing 30 days. Shared by history and anomalies.
func queryRange(r *http.Request) (from, to time.Time, err error) {
	to = time.Now()
	from = to.AddDate(0, 0, -30)

	if value := r.URL.Query().Get("from"); value != "" {
		if from, err = time.Parse(time.RFC3339, value); err != nil {
			return from, to, errors.New("invalid 'from' date")
		}
	}
	if value := r.URL.Query().Get("to"); value != "" {
		if to, err = time.Parse(time.RFC3339, value); err != nil {
			return from, to, errors.New("invalid 'to' date")
		}
	}
	if from.After(to) {
		return from, to, errors.New("'from' must be before 'to'")
	}
	return from, to, nil
}

func history(db *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid satellite id", http.StatusBadRequest)
			return
		}

		from, to, err := queryRange(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
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

func anomalies(db *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid satellite id", http.StatusBadRequest)
			return
		}

		from, to, err := queryRange(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		history, err := db.GetHistory(id, from, to)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		features := feature.Compute(history)
		candidates := detect.Detect(id, features, detect.DefaultConfig())
		if candidates == nil {
			candidates = []detect.Candidate{} // encode as [], not null
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(candidates); err != nil {
			log.Printf("failed to encode anomalies: %v", err)
		}
	}
}

func main() {
	err := godotenv.Load()
	util.Check(err)

	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL - not found")
	}

	db, err := store.NewStore(connStr)
	util.Check(err)
	log.Println("API server starting on :8090")
	http.HandleFunc("GET /api/satellites", satellites(db))
	http.HandleFunc("GET /api/satellites/{id}/history", history(db))
	http.HandleFunc("GET /api/satellites/{id}/anomalies", anomalies(db))

	log.Fatal(http.ListenAndServe(":8090", logging(http.DefaultServeMux)))
}
