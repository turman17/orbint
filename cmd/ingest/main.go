package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/turman17/orbint/internal/celestrak"
	"github.com/turman17/orbint/internal/store"
)

func ingest(client *celestrak.Client, db *store.Store) {
	tles, err := client.FetchGroup("stations")
	if err != nil {
		log.Printf("fetch failed: %v", err)
		return
	}

	err = db.InsertTLEs(tles)
	if err != nil {
		log.Printf("insert failed: %v", err)
		return
	}

	log.Printf("Ingested %d TLEs at %s", len(tles), time.Now().Format(time.RFC3339))
}

func main() {
	godotenv.Load()

	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL - not found")
	}

	db, err := store.NewStore(connStr)
	if err != nil {
		log.Fatal(err)
	}

	client := celestrak.NewClient()

	fmt.Println("Starting ingestion loop (every 2 hours)...")
	ingest(client, db)

	ticker := time.NewTicker(2 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		ingest(client, db)
	}
}
