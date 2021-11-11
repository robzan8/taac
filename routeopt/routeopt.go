package routeopt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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
				Type        string  `json:"type"`
				ServiceId   string  `json:"id"`
				Address     Address `json:"address"`
				ArrivalTime int64   `json:"arr_time"`
				EndTime     int64   `json:"end_time"`
			} `json:"activities"`
		} `json:"routes"`
	} `json:"solution"`
}

func Solve(prob Problem, key string) (Solution, error) {
	var s Solution
	body, err := json.Marshal(&prob)
	if err != nil {
		return s, err
	}
	postUrl := "https://graphhopper.com/api/1/vrp?key=" + key
	resp, err := http.Post(postUrl, "application/json", bytes.NewReader(body))
	if err != nil {
		return s, err
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return s, err
	}
	if resp.StatusCode != http.StatusOK {
		return s, fmt.Errorf("Unexpected response with code %d:\n%s", resp.StatusCode, body)
	}
	err = json.Unmarshal(body, &s)
	return s, err
}

func SolutionToTab(s Solution) [][]string {
	tab := [][]string{{"vehicle id", "activity type", "service id",
		"address", "latitude", "longitude", "time"}}
	for _, route := range s.Solution.Routes {
		for _, act := range route.Activities {
			unixTime := act.ArrivalTime
			if unixTime == 0 {
				unixTime = act.EndTime
			}
			lat := fmt.Sprintf("%f", act.Address.Lat)
			lon := fmt.Sprintf("%f", act.Address.Lon)
			tab = append(tab, []string{route.VehicleId, act.Type, act.ServiceId,
				act.Address.Id, lat, lon, formatHourMin(unixTime),
			})
		}
	}
	return tab
}

func formatHourMin(unixTime int64) string {
	return time.Unix(unixTime, 0).Format("15:04")
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

func ParseVehicles(tab [][]string) ([]Vehicle, error) {
	var vs []Vehicle
	for i := 1; i < len(tab); i++ {
		rec := tab[i]
		if len(rec) != 7 {
			return vs, fmt.Errorf("Line in vehicles csv must have 7 entries")
		}
		var v Vehicle
		v.Id = rec[0]
		v.Type = rec[1]
		if !supportedVehicleTypes[v.Type] {
			return vs, fmt.Errorf("Unsupported vehicle type: %s", v.Type)
		}
		v.StartAddress.Id = rec[2]
		var err error
		v.StartAddress.Lat, err = strconv.ParseFloat(rec[3], 64)
		if err != nil {
			return vs, fmt.Errorf("Invalid float as latitude: %s", rec[3])
		}
		v.StartAddress.Lon, err = strconv.ParseFloat(rec[4], 64)
		if err != nil {
			return vs, fmt.Errorf("Invalid float as longitude: %s", rec[4])
		}
		v.EarliestStart, err = unixTime(rec[5])
		if err != nil {
			return vs, err
		}
		v.LatestEnd, err = unixTime(rec[6])
		if err != nil {
			return vs, err
		}
		vs = append(vs, v)
	}
	return vs, nil
}

// hourMin is in the format "23:59"
func unixTime(hourMin string) (int64, error) {
	var hour, min int64
	_, err := fmt.Sscanf(hourMin, "%d:%d", &hour, &min)
	if err != nil || hour < 0 || hour > 23 || min < 0 || min > 59 {
		return 0, fmt.Errorf("Wrongly formatted time: %s", hourMin)
	}
	return (hour*60 + min) * 60, nil
}

type Service struct {
	Id          string       `json:"id"`
	Address     Address      `json:"address"`
	Size        [1]int       `json:"size"`
	Duration    int64        `json:"duration"`
	TimeWindows []TimeWindow `json:"time_windows,omitempty"`
}

type TimeWindow struct {
	Earliest int64 `json:"earliest"`
	Latest   int64 `json:"latest"`
}

const servicesDuration = 10 * 60 // 10min

func ParseServices(tab [][]string) ([]Service, error) {
	var ss []Service
	for i := 1; i < len(tab); i++ {
		rec := tab[i]
		if len(rec) != 7 {
			return ss, fmt.Errorf("Line in services csv must have 7 entries")
		}
		var s Service
		s.Duration = servicesDuration
		s.Id = rec[0]
		var err error
		s.Size[0], err = strconv.Atoi(rec[1])
		if err != nil || s.Size[0] < 0 {
			return ss, fmt.Errorf("Invalid integer as service size: %s", rec[1])
		}
		s.Address.Id = rec[2]
		s.Address.Lat, err = strconv.ParseFloat(rec[3], 64)
		if err != nil {
			return ss, fmt.Errorf("Invalid float as latitude: %s", rec[3])
		}
		s.Address.Lon, err = strconv.ParseFloat(rec[4], 64)
		if err != nil {
			return ss, fmt.Errorf("Invalid float as longitude: %s", rec[4])
		}
		if rec[5] != "" && rec[6] != "" {
			earliest, err := unixTime(rec[5])
			if err != nil {
				return ss, err
			}
			latest, err := unixTime(rec[6])
			if err != nil {
				return ss, err
			}
			s.TimeWindows = []TimeWindow{{
				Earliest: earliest,
				Latest:   latest,
			}}
		}
		ss = append(ss, s)
	}
	return ss, nil
}
