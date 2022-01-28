package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
)

type location struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lng"`
}

var (
	cache   = make(map[string]location)
	cacheMu sync.Mutex
)

const cacheCap = 5000

func load(addr string) location {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	return cache[addr]
}

func store(addr string, loc location) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	if len(cache) < cacheCap {
		cache[addr] = loc
	} else {
		cache = map[string]location{addr: loc}
	}
}

func GeocodeAddress(addr string) (lat, lon float64, err error) {
	loc := load(addr)
	if loc != (location{}) {
		return loc.Lat, loc.Lon, nil
	}

	loc, err = geocodeAddressApi(addr)
	if err != nil {
		return
	}
	store(addr, loc)
	return loc.Lat, loc.Lon, nil
}

func geocodeAddressApi(addr string) (loc location, err error) {
	base := "https://maps.googleapis.com/maps/api/geocode/json"
	queryUrl := fmt.Sprintf("%s?address=%s&key=%s", base, url.QueryEscape(addr), geocodeKey)
	var resp *http.Response
	resp, err = http.Get(queryUrl)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("Geocode query responded with status %d", resp.StatusCode)
		return
	}

	var res geocodingResult
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&res)
	if err != nil {
		return
	}
	if res.ErrorMsg != "" {
		err = fmt.Errorf("Error geocoding address %q: %s", addr, res.ErrorMsg)
		return
	}
	if len(res.Results) == 0 {
		err = fmt.Errorf("No geocode results for address %q", addr)
		return
	}
	return res.Results[0].Geometry.Location, nil
}

type geocodingResult struct {
	ErrorMsg string `json:"error_message"`
	Results  []struct {
		Geometry struct {
			Location location `json:"location"`
		} `json:"geometry"`
	} `json:"results"`
}
