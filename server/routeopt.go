package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

const (
	PickupPrepTime   = 15 * 60 // 15min
	DeliveryPrepTime = 5 * 60  // 5min

	ActivityTypeStart   = "start"
	ActivityTypeEnd     = "end"
	ActivityTypePickup  = "pickupShipment"
	ActivityTypeDeliver = "deliverShipment"
)

type Problem struct {
	Vehicles     []Vehicle     `json:"vehicles"`
	VehicleTypes []VehicleType `json:"vehicle_types"`
	Shipments    []Shipment    `json:"shipments"`
}

type Vehicle struct {
	Id            string  `json:"vehicle_id"`
	Type          string  `json:"type_id"`
	StartAddress  Address `json:"start_address"`
	EarliestStart int64   `json:"earliest_start"`
	LatestEnd     int64   `json:"latest_end"`
}

type Address struct {
	Str string  `json:"location_id"`
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type VehicleType struct {
	Id          string  `json:"type_id"`
	Capacity    [1]int  `json:"capacity"`
	Profile     string  `json:"profile"`
	SpeedFactor float64 `json:"speed_factor"`
}

const CargoBikeId = "cargo-bike"

var CargoBikeType = VehicleType{
	Id:          CargoBikeId,
	Capacity:    [1]int{1000},
	Profile:     "bike",
	SpeedFactor: 0.7,
}

type Shipment struct {
	Id       string   `json:"id"`
	Size     [1]int   `json:"size"`
	Pickup   Delivery `json:"pickup"`
	Delivery Delivery `json:"delivery"`
	Priority int      `json:"priority,omitempty"`
}

type Delivery struct {
	Address     Address      `json:"address"`
	PrepTime    int64        `json:"preparation_time"`
	TimeWindows []TimeWindow `json:"time_windows,omitempty"`
}

type TimeWindow struct {
	Earliest int64 `json:"earliest"`
	Latest   int64 `json:"latest"`
}

func CreateProblem(vehicles []Vehicle, shipments []Shipment) Problem {
	return Problem{
		Vehicles:     vehicles,
		VehicleTypes: []VehicleType{CargoBikeType},
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

func Solve(prob Problem) (Solution, error) {
	var s Solution
	body, err := json.Marshal(&prob)
	if err != nil {
		return s, err
	}
	postUrl := "https://graphhopper.com/api/1/vrp?key=" + routeoptKey
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
