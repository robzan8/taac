package routeopt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
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
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
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

func ParseVehicles(tab [][]string, now time.Time) []Vehicle {
	var vs []Vehicle
	for i := 1; i < len(tab); i++ {
		rec := tab[i]
		if len(rec) != 7 {
			log.Fatal("Line in vehicles csv must have 7 entries")
		}
		var v Vehicle
		v.Id = rec[0]
		v.Type = rec[1]
		if !supportedVehicleTypes[v.Type] {
			log.Fatalf("Unsupported vehicle type: %s", v.Type)
		}
		v.StartAddress.Id = rec[2]
		var err error
		v.StartAddress.Lat, err = strconv.ParseFloat(rec[3], 64)
		if err != nil {
			log.Fatalf("Invalid float as latitude: %s", rec[3])
		}
		v.StartAddress.Lon, err = strconv.ParseFloat(rec[4], 64)
		if err != nil {
			log.Fatalf("Invalid float as longitude: %s", rec[4])
		}
		v.EarliestStart = unixTimeStamp(rec[5], now)
		v.LatestEnd = unixTimeStamp(rec[6], now)
		vs = append(vs, v)
	}
	return vs
}

// hourMin is in the format "23:59"
func unixTimeStamp(hourMin string, now time.Time) int64 {
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

func ParseServices(tab [][]string, now time.Time) []Service {
	var ss []Service
	for i := 1; i < len(tab); i++ {
		rec := tab[i]
		if len(rec) != 7 {
			log.Fatal("Line in services csv must have 7 entries")
		}
		var s Service
		s.Id = rec[0]
		var err error
		s.Size[0], err = strconv.Atoi(rec[1])
		if err != nil || s.Size[0] < 0 {
			log.Fatalf("Invalid integer as service size: %s", rec[1])
		}
		s.Address.Id = rec[2]
		s.Address.Lat, err = strconv.ParseFloat(rec[3], 64)
		if err != nil {
			log.Fatalf("Invalid float as latitude: %s", rec[3])
		}
		s.Address.Lon, err = strconv.ParseFloat(rec[4], 64)
		if err != nil {
			log.Fatalf("Invalid float as longitude: %s", rec[4])
		}
		s.TimeWindows[0].Earliest = unixTimeStamp(rec[5], now)
		s.TimeWindows[0].Latest = unixTimeStamp(rec[6], now)
		ss = append(ss, s)
	}
	return ss
}
