package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
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

	GeocodeTable(tab, key)

	w := csv.NewWriter(os.Stdout)
	err = w.WriteAll(tab)
	if err != nil {
		log.Fatal(err)
	}
}

const addressCol = 2

func GeocodeTable(tab [][]string, key string) {
	for i, row := range tab {
		latStr := "latitude"
		lonStr := "longitude"
		if i > 0 {
			lat, lon := GeocodeAddress(row[addressCol], key)
			latStr = fmt.Sprintf("%f", lat)
			lonStr = fmt.Sprintf("%f", lon)
		}
		j := addressCol + 1
		// Insert cells latStr and lonStr at index j
		row = append(row[0:j+2], row[j:]...)
		row[j] = latStr
		row[j+1] = lonStr
		tab[i] = row
	}
}

type GeocodeResults struct {
	Results []struct {
		Geometry struct {
			Location struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"location"`
		} `json:"geometry"`
	} `json:"results"`
}

func GeocodeAddress(addr, key string) (lat, lon float64) {
	base := "https://maps.googleapis.com/maps/api/geocode/json"
	queryUrl := fmt.Sprintf("%s?address=%s&key=%s", base, url.QueryEscape(addr), key)
	resp, err := http.Get(queryUrl)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Geocode query responded with status %d\n", resp.StatusCode)
	}

	var res GeocodeResults
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&res)
	if err != nil {
		log.Fatal(err)
	}
	if len(res.Results) == 0 {
		log.Fatalf("No geocode results for address: %s\n", addr)
	}
	loc := res.Results[0].Geometry.Location
	log.Printf("Address %q geocoded as %f, %f\n", addr, loc.Lat, loc.Lng)
	return loc.Lat, loc.Lng
}
