package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
)

const (
	addressCol     = 2
	shipAddressCol = 5
)

func main() {
	log.SetFlags(0)
	isShipment := flag.Bool("shipment", false, "tells if input is a shipment table")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		log.Fatal(`Google api key not provided.
Usage: geocode [-shipment] your-google-api-key <vehicles_or_shipments.csv >result.csv`)
	}
	key := args[0]

	r := csv.NewReader(os.Stdin)
	tab, err := r.ReadAll()
	if err != nil {
		log.Fatal(err)
	}

	err = GeocodeColumn(tab, addressCol, key, log.Default())
	if err != nil {
		log.Fatal(err)
	}
	if *isShipment {
		err = GeocodeColumn(tab, shipAddressCol, key, log.Default())
		if err != nil {
			log.Fatal(err)
		}
	}

	w := csv.NewWriter(os.Stdout)
	err = w.WriteAll(tab)
	if err != nil {
		log.Fatal(err)
	}
}

var cache = make(map[string]location)

type Logger interface {
	Printf(format string, v ...interface{})
}

func GeocodeColumn(tab [][]string, col int, key string, logger Logger) error {
	for i, row := range tab {
		lat := "lat"
		lon := "lon"
		if i > 0 {
			addr := row[col]
			loc, ok := cache[addr]
			if !ok {
				var err error
				loc.Lat, loc.Lng, err = GeocodeAddress(addr, key)
				if err != nil {
					return err
				}
				if logger != nil {
					logger.Printf("Address %q geocoded as %f, %f\n", addr, loc.Lat, loc.Lng)
				}
				cache[addr] = loc
			}
			lat = fmt.Sprintf("%f", loc.Lat)
			lon = fmt.Sprintf("%f", loc.Lng)
		}
		j := col + 1
		// Insert cells lat and lon at index j
		row = append(row, "", "")
		copy(row[j+2:], row[j:])
		row[j] = lat
		row[j+1] = lon
		tab[i] = row
	}
	return nil
}

type geocodeResults struct {
	Results []struct {
		Geometry struct {
			Location location `json:"location"`
		} `json:"geometry"`
	} `json:"results"`
}

type location struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

func GeocodeAddress(addr, key string) (lat, lon float64, err error) {
	base := "https://maps.googleapis.com/maps/api/geocode/json"
	queryUrl := fmt.Sprintf("%s?address=%s&key=%s", base, url.QueryEscape(addr), key)
	resp, err := http.Get(queryUrl)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("Geocode query responded with status %d\n", resp.StatusCode)
	}

	var res geocodeResults
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&res)
	if err != nil {
		return 0, 0, err
	}
	if len(res.Results) == 0 {
		return 0, 0, fmt.Errorf("No geocode results for address: %s\n", addr)
	}
	loc := res.Results[0].Geometry.Location
	return loc.Lat, loc.Lng, nil
}
