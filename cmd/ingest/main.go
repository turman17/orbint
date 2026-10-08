package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/turman17/orbint/internal/celestrak"
	"github.com/turman17/orbint/internal/store"
)

// defaultGroups is a varied slice of the catalog, roughly a thousand
// objects: crewed stations and the brightest objects (LEO), polar weather
// satellites (sun-synchronous LEO), the Iridium constellation (LEO, 86°),
// every navigation constellation (MEO plus inclined GEO), the GEO belt,
// science missions, and whatever launched in the last 30 days.
// Group names: https://celestrak.org/NORAD/elements/
const defaultGroups = "stations,visual,weather,noaa,iridium-NEXT,gnss,geo,science,last-30-days"

// groups returns the CelesTrak groups to ingest from CELESTRAK_GROUPS
// (comma-separated), falling back to defaultGroups.
func groups() []string {
	raw := os.Getenv("CELESTRAK_GROUPS")
	if strings.TrimSpace(raw) == "" {
		raw = defaultGroups
	}
	var out []string
	for _, g := range strings.Split(raw, ",") {
		if g = strings.TrimSpace(g); g != "" {
			out = append(out, g)
		}
	}
	return out
}

func ingest(client *celestrak.Client, db *store.Store, groups []string) {
	total := 0
	for _, group := range groups {
		tles, err := client.FetchGroup(group)
		if err != nil {
			log.Printf("fetch %s failed: %v", group, err)
			continue
		}

		// Objects appear in several groups (ISS is in stations and visual);
		// the (norad_cat_id, epoch) conflict rule in InsertTLEs dedupes them.
		if err := db.InsertTLEs(tles); err != nil {
			log.Printf("insert %s failed: %v", group, err)
			continue
		}
		total += len(tles)
	}

	log.Printf("Ingested %d TLEs from %d groups at %s", total, len(groups), time.Now().Format(time.RFC3339))
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
	gs := groups()

	fmt.Printf("Starting ingestion loop (every 2 hours) for groups %v...\n", gs)
	ingest(client, db, gs)

	ticker := time.NewTicker(2 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		ingest(client, db, gs)
	}
}
