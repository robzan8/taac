package routeopt

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

type Problem struct {
	Vehicles     []Vehicle     `json:"vehicles"`
	VehicleTypes []VehicleType `json:"vehicle_types"`
	Services     []Service     `json:"services"`
}

func CreateProblem(vehicles []Vehicle, services []Service) Problem {
	return Problem{
		Vehicles:     vehicles,
		VehicleTypes: []VehicleType{cargoBikeType},
		Services:     services,
	}
}

type Solution struct {
	Solution struct {
		Routes []struct {
			VehicleId  string `json:"vehicle_id"`
			Activities []struct {
				Type    string  `json:"type"`
				Address Address `json:"address"`
				Arrival int64   `json:"arr_date_time"`
				Service Service `json:"service"`
			} `json:"activities"`
		} `json:"routes"`
	} `json:"solution"`
}

func Solve(prob Problem, key string) {
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

func ReadVehicles(fileName, key string) []Vehicle {
	f, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	var v []Vehicle
	r := csv.NewReader(f)
	for i := 0; true; i++ {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		// Skip empty lines and possible header.
		if len(rec) == 0 || i == 0 && rec[0] == "id" {
			continue
		}
		v = append(v, decodeVehicle(rec, key))
	}
	return v
}

func decodeVehicle(rec []string, key string) Vehicle {
	if len(rec) != 5 {
		log.Fatal("Line in vehicles csv must have 5 entries")
	}
	var v Vehicle
	v.Id = rec[0]
	v.Type = rec[1]
	if !supportedVehicleTypes[v.Type] {
		log.Fatalf("Unsupported vehicle type: %s", v.Type)
	}
	v.StartAddress = GeocodeAddress(rec[2], key)
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

func GeocodeAddress(s, key string) Address {
	base := "https://graphhopper.com/api/1/geocode"
	q := url.QueryEscape(s)
	queryUrl := fmt.Sprintf("%s?q=%s&locale=it&debug=true&key=%s", base, q, key)
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
	log.Printf("Address %q geocoded as %f, %f\n", s, p.Lat, p.Lng)
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
	Id          string  `json:"id"`
	Address     Address `json:"address"`
	Size        [1]int  `json:"size"`
	TimeWindows [1]struct {
		Earliest int64 `json:"earliest"`
		Latest   int64 `json:"latest"`
	} `json:"time_windows"`
}

func ReadServices(fileName, key string) []Service {
	f, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	var s []Service
	r := csv.NewReader(f)
	for i := 0; true; i++ {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		// Skip empty lines and possible header.
		if len(rec) == 0 || i == 0 && rec[0] == "id" {
			continue
		}
		s = append(s, decodeService(rec, key))
	}
	return s
}

func decodeService(rec []string, key string) Service {
	if len(rec) != 5 {
		log.Fatal("Line in services csv must have 5 entries")
	}
	var s Service
	s.Id = rec[0]
	s.Address = GeocodeAddress(rec[1], key)
	size, err := strconv.Atoi(rec[2])
	if err != nil || size < 0 {
		log.Fatalf("Invalid integer as service size: %s", rec[2])
	}
	s.Size[0] = size
	s.TimeWindows[0].Earliest = unixTimeStamp(rec[3])
	s.TimeWindows[0].Latest = unixTimeStamp(rec[4])
	return s
}
