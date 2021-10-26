package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

var key string

func main() {
	log.SetFlags(0)

	args := os.Args[1:]
	if len(args) < 3 {
		log.Fatal(`Not enough input arguments provided.
Usage: hop vehicles.csv services.csv your-graphhopper-api-key`)
	}
	key = args[2]

	problem := RouteProblem{
		Vehicles:     readVehicles(args[0]),
		VehicleTypes: []VehicleType{cargoBikeType},
		Services:     readServices(args[1]),
	}
	solve(problem)
}

type Vehicle struct {
	Id            string  `json:"vehicle_id"`
	Type          string  `json:"type_id"`
	StartAddress  Address `json:"start_address"`
	EarliestStart int64   `json:"earliest_start"`
	LatestEnd     int64   `json:"latest_end"`
}

type Address struct {
	Id  string  `json:"location_id"`
	Lon float64 `json:"lon"`
	Lat float64 `json:"lat"`
}

type VehicleType struct {
	Id       string `json:"type_id"`
	Capacity [1]int `json:"capacity"`
	Profile  string `json:"profile"`
}

var supportedVehicleTypes = map[string]bool{"cargo-bike": true}

var cargoBikeType = VehicleType{
	Id:       "cargo-bike",
	Capacity: [1]int{10},
	Profile:  "bike",
}

func readVehicles(fileName string) []Vehicle {
	f, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	var v []Vehicle
	r := csv.NewReader(f)
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		v = append(v, decodeVehicle(rec))
	}
	return v
}

func decodeVehicle(rec []string) Vehicle {
	if len(rec) != 5 {
		log.Fatal("Line in vehicles csv must have 5 entries")
	}
	var v Vehicle
	v.Id = rec[0]
	v.Type = rec[1]
	if !supportedVehicleTypes[v.Type] {
		log.Fatalf("Unsupported vehicle type: %s", v.Type)
	}
	v.StartAddress = geocodeAddress(rec[2])
	v.EarliestStart = unixTimeStamp(rec[3])
	v.LatestEnd = unixTimeStamp(rec[4])
	return v
}

type GeocodeHits struct {
	Hits []struct {
		Point struct {
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		} `json:"point"`
	} `json:"hits"`
}

func geocodeAddress(s string) Address {
	base := "https://graphhopper.com/api/1/geocode"
	q := url.QueryEscape(s)
	queryUrl := fmt.Sprintf("%s?q=%s&locale=it&debug=true&key=%s", base, q, key)
	log.Println(queryUrl)
	resp, err := http.Get(queryUrl)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Geocode query responded with status %d\n", resp.StatusCode)
	}

	var hits GeocodeHits
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&hits)
	if err != nil {
		log.Fatal(err)
	}
	if len(hits.Hits) == 0 {
		log.Fatalf("No geocode hits for address: %s\n", s)
	}
	p := hits.Hits[0].Point
	return Address{Id: s, Lat: p.Lat, Lon: p.Lng}
}

var now = time.Now()

// hourMin is in the format "23:59"
func unixTimeStamp(hourMin string) int64 {
	var hour, min int
	_, err := fmt.Sscanf(hourMin, "%d:%d", &hour, &min)
	if err != nil || hour < 0 || hour > 23 || min < 0 || min > 59 {
		log.Fatalf("Wrongly formatted time: %s", hourMin)
	}
	year, month, day := now.Date()
	// If we are running the script after 12:00,
	// we are planning tomorrow's schedule.
	if hour >= 12 {
		day++
	}
	return time.Date(year, month, day, hour, min, 0, 0, time.Local).Unix()
}

type Service struct {
	Id         string  `json:"id"`
	Address    Address `json:"address"`
	Size       [1]int  `json:"size"`
	TimeWindow struct {
		Earliest int64 `json:"earliest"`
		Latest   int64 `json:"latest"`
	} `json:"time_windows"`
}

func readServices(fileName string) []Service {
	f, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	var s []Service
	r := csv.NewReader(f)
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		s = append(s, decodeService(rec))
	}
	return s
}

func decodeService(rec []string) Service {
	if len(rec) != 5 {
		log.Fatal("Line in services csv must have 5 entries")
	}
	var s Service
	s.Id = rec[0]
	s.Address = geocodeAddress(rec[1])
	size, err := strconv.Atoi(rec[2])
	if err != nil || size < 0 {
		log.Fatalf("Invalid integer as service size: %s", rec[2])
	}
	s.Size[0] = size
	s.TimeWindow.Earliest = unixTimeStamp(rec[3])
	s.TimeWindow.Latest = unixTimeStamp(rec[4])
	return s
}

type RouteProblem struct {
	Vehicles     []Vehicle     `json:"vehicles"`
	VehicleTypes []VehicleType `json:"vehicle_types"`
	Services     []Service     `json:"services"`
}

/*type RouteSolution struct {
	Solution struct {
		Routes []struct {
			VehicleId  string `json:"vehicle_id"`
			Activities []struct {
				Type string `json:"type"`
				// todo
			} `json:"activities"`
		} `json:"routes"`
	} `json:"solution"`
}*/

func solve(prob RouteProblem) {
	body, err := json.Marshal(&prob)
	if err != nil {
		log.Fatal(err)
	}
	postUrl := "https://graphhopper.com/api/1/vrp?key=" + key
	resp, err := http.Post(postUrl, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Unexpected response with code %d:\n%s", resp.StatusCode, body)
	}
	f, err := os.Create("./solution.json")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	_, err = f.Write(body)
	if err != nil {
		log.Fatal(err)
	}
}
