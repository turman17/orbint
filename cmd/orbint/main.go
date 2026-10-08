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

func main() {
	err := godotenv.Load()
	check(err)

	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL - not found")
	}

	db, err := store.NewStore(connStr)
	check(err)

	client := celestrak.NewClient()
	tles, err := client.FetchGroup("stations")
	check(err)

	db.InsertTLEs(tles)
	check(err)
	fmt.Printf("Inserted %d TLEs\n", len(tles))

	fmt.Printf("\n-----------------\n")

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Now()
	history, err := db.GetHistory(25544, from, to)
	check(err)
	fmt.Printf("ISS history: %d records\n", len(history))
	for _, t := range history {
		fmt.Printf("  %s  inc=%.4f  ecc=%.7f\n", t.Epoch.Format("2006-01-02"), t.Inclination, t.Eccentricity)
	}
}
