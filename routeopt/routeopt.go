package routeopt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
)

type Problem struct {
	Vehicles     []Vehicle     `json:"vehicles"`
	VehicleTypes []VehicleType `json:"vehicle_types"`
	Shipments    []Shipment    `json:"shipments"`
}

func CreateProblem(vehicles []Vehicle, shipments []Shipment) Problem {
	return Problem{
		Vehicles:     vehicles,
		VehicleTypes: []VehicleType{cargoBikeType},
		Shipments:    shipments,
	}
}

type Solution struct {
	Solution struct {
		Routes []struct {
			VehicleId  string `json:"vehicle_id"`
			Activities []struct {
				Type        string  `json:"type"`
				ShipmentId  string  `json:"id"`
				Address     Address `json:"address"`
				ArrivalTime int64   `json:"arr_time"`
				EndTime     int64   `json:"end_time"`
			} `json:"activities"`
		} `json:"routes"`
		Unassigned struct {
			Shipments []string `json:"shipments"`
		} `json:"unassigned"`
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

func SolveTables(vehiclesTab, shipmentsTab [][]string, key string) ([][]string, error) {
	vehicles, err := ParseVehicles(vehiclesTab)
	if err != nil {
		return nil, err
	}
	shipments, err := ParseShipments(shipmentsTab)
	if err != nil {
		return nil, err
	}
	problem := CreateProblem(vehicles, shipments)
	solution, err := Solve(problem, key)
	if err != nil {
		return nil, err
	}
	return SolutionToTab(solution), nil
}

func SolutionToTab(s Solution) [][]string {
	tab := [][]string{{"vehicle id", "activity type", "shipment id",
		"address", "lat", "lon", "time"}}
	for _, route := range s.Solution.Routes {
		for _, act := range route.Activities {
			unixTime := act.ArrivalTime
			if unixTime == 0 {
				unixTime = act.EndTime
			}
			lat := fmt.Sprintf("%f", act.Address.Lat)
			lon := fmt.Sprintf("%f", act.Address.Lon)
			tab = append(tab, []string{route.VehicleId, act.Type, act.ShipmentId,
				act.Address.Id, lat, lon, formatHourMin(unixTime),
			})
		}
	}
	for _, shipId := range s.Solution.Unassigned.Shipments {
		tab = append(tab, []string{"", "unassigned", shipId})
	}
	return tab
}

func formatHourMin(unixTime int64) string {
	minutes := unixTime / 60
	min := minutes % 60
	hour := (minutes / 60) % 24
	format := "%d:%d"
	if min < 10 {
		format = "%d:0%d"
	}
	return fmt.Sprintf(format, hour, min)
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
	Capacity: [1]int{1000},
	Profile:  "bike",
}

func ParseVehicles(tab [][]string) ([]Vehicle, error) {
	var vs []Vehicle
	for i := 1; i < len(tab); i++ {
		rec := tab[i]
		if len(rec) != 7 {
			return nil, fmt.Errorf("Line in vehicles csv must have 7 entries")
		}
		var v Vehicle
		v.Id = rec[0]
		v.Type = rec[1]
		if !supportedVehicleTypes[v.Type] {
			return nil, fmt.Errorf("Unsupported vehicle type: %s", v.Type)
		}
		var err error
		v.StartAddress, err = parseAddress(rec[2:])
		if err != nil {
			return nil, err
		}
		v.EarliestStart, err = unixTime(rec[5])
		if err != nil {
			return nil, err
		}
		v.LatestEnd, err = unixTime(rec[6])
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
	}
	return vs, nil
}

func parseAddress(rec []string) (Address, error) {
	var a Address
	a.Id = rec[0]
	var err error
	a.Lat, err = strconv.ParseFloat(rec[1], 64)
	if err != nil {
		return Address{}, fmt.Errorf("Invalid float as latitude: %s", rec[1])
	}
	a.Lon, err = strconv.ParseFloat(rec[2], 64)
	if err != nil {
		return Address{}, fmt.Errorf("Invalid float as longitude: %s", rec[2])
	}
	return a, nil
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

type Shipment struct {
	Id       string   `json:"id"`
	Size     [1]int   `json:"size"`
	Pickup   Delivery `json:"pickup"`
	Delivery Delivery `json:"delivery"`
}

type Delivery struct {
	Address  Address `json:"address"`
	PrepTime int64   `json:"preparation_time"`
}

const deliveryPrepTime = 10 * 60 // 10min

func ParseShipments(tab [][]string) ([]Shipment, error) {
	var ss []Shipment
	for i := 1; i < len(tab); i++ {
		rec := tab[i]
		if len(rec) != 8 {
			return nil, fmt.Errorf("Line in shipments csv must have 8 entries")
		}
		var s Shipment
		s.Id = rec[0]
		var err error
		s.Size[0], err = strconv.Atoi(rec[1])
		if err != nil || s.Size[0] < 0 {
			return nil, fmt.Errorf("Invalid integer as shipment size: %s", rec[1])
		}
		s.Pickup.Address, err = parseAddress(rec[2:])
		if err != nil {
			return nil, err
		}
		s.Pickup.PrepTime = deliveryPrepTime
		s.Delivery.Address, err = parseAddress(rec[5:])
		if err != nil {
			return nil, err
		}
		s.Delivery.PrepTime = deliveryPrepTime
		ss = append(ss, s)
	}
	return ss, nil
}
