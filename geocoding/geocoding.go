package geocoding

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type Logger interface {
	Printf(format string, v ...interface{})
}

const addressCol = 2

func GeocodeTable(tab [][]string, key string, logger Logger) error {
	for i, row := range tab {
		latStr := "latitude"
		lonStr := "longitude"
		if i > 0 {
			lat, lon, err := GeocodeAddress(row[addressCol], key)
			if err != nil {
				return err
			}
			if logger != nil {
				logger.Printf("Address %q geocoded as %f, %f\n", row[addressCol], lat, lon)
			}
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
	return nil
}

type geocodeResults struct {
	Results []struct {
		Geometry struct {
			Location struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"location"`
		} `json:"geometry"`
	} `json:"results"`
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
