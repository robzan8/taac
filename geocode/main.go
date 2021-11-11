package main

import (
	"encoding/csv"
	"log"
	"os"

	"github.com/robzan8/hop/geocoding"
)

func main() {
	log.SetFlags(0)

	args := os.Args[1:]
	if len(args) < 1 {
		log.Fatal(`Google api key not provided.
Usage: geocode your-google-api-key <vehicles_or_services.csv >result.csv`)
	}
	key := args[0]

	r := csv.NewReader(os.Stdin)
	tab, err := r.ReadAll()
	if err != nil {
		log.Fatal(err)
	}

	err = geocoding.GeocodeTable(tab, key, log.Default())
	if err != nil {
		log.Fatal(err)
	}

	w := csv.NewWriter(os.Stdout)
	err = w.WriteAll(tab)
	if err != nil {
		log.Fatal(err)
	}
}
