package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"sync"
)

type shipmentData struct {
	Id     string `json:"id"`
	User   string `json:"user_data_ref_id"`
	Schema string `json:"schema_id"`
	Data   struct {
		Size            int    `json:"size"`
		PickupAddress   string `json:"pickup_address"`
		DeliveryAddress string `json:"delivery_address"`
		Notes           string `json:"notes"`

		RiderId        string `json:"rider_id"`
		ShipmentDay    string `json:"shipment_day"`
		PickupTime     string `json:"pickup_time"`
		DeliveryTime   string `json:"delivery_time"`
		DeliveryStatus string `json:"delivery_status"`
	} `json:"data"`
}

const (
	deliveryStatusToBeScheduled = "to_be_scheduled"
	deliveryStatusScheduled     = "scheduled"
	deliveryStatusDelivered     = "delivered"
	deliveryStatusCanceled      = "canceled"
)

type riderData struct {
	Id   string `json:"id"`
	Data struct {
		Name          string `json:"name"`
		VehicleTypeId string `json:"vehicle_type_id"`
		StartAddress  string `json:"start_address"`
		EarliestStart string `json:"earliest_start"`
		LatestEnd     string `json:"latest_end"`
	} `json:"data"`
}

const graphqlUrl = "https://hkfuuoyvxplvytbyarwl.nhost.run/v1/graphql"

var (
	scheduleMu   sync.Mutex
	dateRegex, _ = regexp.Compile(`\d\d\d\d-[0-1]\d-[0-3]\d`)
)

func scheduleEndpoint(w http.ResponseWriter, req *http.Request) {
	setAllowOrigins(w.Header())

	switch req.Method {
	case http.MethodOptions:
		// OK
	case http.MethodGet:
		scheduleGet(w, req)
	default:
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Unsupported method %s", req.Method)
	}
}

func scheduleGet(w http.ResponseWriter, req *http.Request) {
	scheduleMu.Lock()
	var err error // beware of shadowing
	defer func() {
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprintf(w, "%s", err)
		}
		scheduleMu.Unlock()
	}()

	schedDate := req.FormValue("date")
	if !dateRegex.MatchString(schedDate) {
		err = fmt.Errorf("date must be in the format 2022-01-31")
		return
	}
	authHeader, err := dinoLogin()
	if err != nil {
		return
	}
	riderData, err := getRiderData(authHeader)
	if err != nil {
		return
	}
	shipData, err := getShipmentData(authHeader)
	if err != nil {
		return
	}

	availRiders := availableRiders(riderData, schedDate, shipData)
	if len(availRiders) == 0 {
		fmt.Fprint(w, "No rider available for the target day")
		return
	}
	shipsToBeSched := shipmentsToBeScheduled(shipData, availRiders)
	if len(shipsToBeSched) == 0 {
		fmt.Fprint(w, "No shipment to be scheduled")
		return
	}

	var vehicles []Vehicle
	for _, r := range availRiders {
		var v Vehicle
		v, err = riderToVehicle(r)
		if err != nil {
			return
		}
		vehicles = append(vehicles, v)
	}
	var ships []Shipment
	for _, data := range shipsToBeSched {
		var s Shipment
		s, err = dataToShipment(data)
		if err != nil {
			return
		}
		ships = append(ships, s)
	}
	problem := CreateProblem(vehicles, ships)
	solution, err := Solve(problem)
	if err != nil {
		return
	}

	shipsById := make(map[string]*shipmentData)
	for i, s := range shipsToBeSched {
		shipsById[s.Id] = &shipsToBeSched[i]
	}
	for _, route := range solution.Solution.Routes {
		riderId := route.VehicleId
		for _, act := range route.Activities {
			switch act.Type {
			case ActivityTypePickup:
				ship := shipsById[act.ShipmentId]
				ship.Data.DeliveryStatus = deliveryStatusScheduled
				ship.Data.RiderId = riderId
				ship.Data.ShipmentDay = schedDate
				pickupTime := act.ArrivalTime
				if pickupTime == 0 {
					pickupTime = act.EndTime
				}
				ship.Data.PickupTime = formatHourMin(pickupTime)
			case ActivityTypeDeliver:
				ship := shipsById[act.ShipmentId]
				deliveryTime := act.ArrivalTime
				if deliveryTime == 0 {
					deliveryTime = act.EndTime
				}
				ship.Data.DeliveryTime = formatHourMin(deliveryTime)
			}
		}
	}
	var schedShips []shipmentData
	for _, s := range shipsToBeSched {
		if s.Data.DeliveryStatus == deliveryStatusScheduled {
			schedShips = append(schedShips, s)
		}
	}
	err = updateShipmentData(authHeader, schedShips)
	if err != nil {
		return
	}
	fmt.Fprint(w, "The following shipments have been scheduled:")
	for _, s := range schedShips {
		fmt.Fprint(w, "\n"+s.Id)
	}
}

func dinoLogin() (authHeader string, err error) {
	const (
		url   = "https://auth.gnucoop.io/api/login"
		appId = "61dbbdd8-7144-45c6-bb91-8497a31849b0"
		auth  = "jVe6r5r3u-0sAHjo1nzAIk6j5JTB1_qZOunuY4oMSSBIXClCKbA-3rJb"
	)
	reqBody := fmt.Sprintf(`{"loginId":%q,"password":%q,"applicationId":%q}`,
		dinoUser, dinoPass, appId)
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("Dino login error: %s", err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", auth)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Dino login error: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Dino login fail")
	}
	var msg struct {
		Token string `json:"token"`
	}
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&msg)
	return "Bearer " + msg.Token, err
}

type QueryErrors struct {
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func queryAndDecode(authHeader, query string, vars map[string]interface{}, dest interface{}) error {
	varsJson := []byte("{}")
	if len(vars) > 0 {
		var err error
		varsJson, err = json.Marshal(vars)
		if err != nil {
			return err
		}
	}
	reqBody := fmt.Sprintf(`{"query":%q,"variables":%s}`, query, varsJson)
	req, err := http.NewRequest(http.MethodPost, graphqlUrl, strings.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", authHeader)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Graphql query not ok, status: %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

func getRiderData(authHeader string) ([]riderData, error) {
	const query = `{
		form_data(
			where: {_and:[
				{schema_id:{_eq:"2ab47e7b-44fa-4e5d-8b31-4f94d7e9a1bd"}},
				{is_deleted:{_eq:false}}
			]},
			limit: 50
		)
		{id data}
	}`
	var msg struct {
		QueryErrors
		Data struct {
			RiderData []riderData `json:"form_data"`
		} `json:"data"`
	}
	err := queryAndDecode(authHeader, query, nil, &msg)
	if err != nil {
		return nil, err
	}
	if len(msg.Errors) > 0 {
		return nil, errors.New(msg.Errors[0].Message)
	}
	return msg.Data.RiderData, nil
}

func getShipmentData(authHeader string) ([]shipmentData, error) {
	const query = `{
		form_data(
			where: {_and:[
				{schema_id:{_eq:"f66e78ba-321f-46fb-aedb-7c21639eff52"}},
				{is_deleted:{_eq:false}}
			]},
			order_by: [{created_at: desc}],
			limit: 500
		)
		{id data}
	}`
	var msg struct {
		QueryErrors
		Data struct {
			ShipData []shipmentData `json:"form_data"`
		} `json:"data"`
	}
	err := queryAndDecode(authHeader, query, nil, &msg)
	if err != nil {
		return nil, err
	}
	if len(msg.Errors) > 0 {
		return nil, errors.New(msg.Errors[0].Message)
	}
	return msg.Data.ShipData, nil
}

func updateShipmentData(authHeader string, ships []shipmentData) error {
	const query = `mutation($shipments: [form_data_insert_input]!) {
		insert_form_data(
			objects: $shipments,
			on_conflict: {
				constraint: form_data_pkey,
				update_columns: [data]
			}
		)
		{
			affected_rows
		}
	}`
	vars := map[string]interface{}{"shipments": ships}
	var msg QueryErrors
	err := queryAndDecode(authHeader, query, vars, &msg)
	if err != nil {
		return err
	}
	if len(msg.Errors) > 0 {
		return errors.New(msg.Errors[0].Message)
	}
	return nil
}

func availableRiders(riders []riderData, day string, ships []shipmentData) []riderData {
	busyRiders := make(map[string]bool)
	for _, s := range ships {
		if s.Data.ShipmentDay == day {
			busyRiders[s.Data.RiderId] = true
		}
	}
	rand.Shuffle(len(riders), func(i, j int) {
		riders[i], riders[j] = riders[j], riders[i]
	})
	const maxNumRiders = 2
	var selected []riderData
	for _, r := range riders {
		if !busyRiders[r.Id] {
			selected = append(selected, r)
			if len(selected) == maxNumRiders {
				break
			}
		}
	}
	return selected
}

func shipmentsToBeScheduled(ships []shipmentData, availRiders []riderData) []shipmentData {
	locations := make(map[string]bool)
	const maxNumLocs = 30
	for _, r := range availRiders {
		locations[r.Data.StartAddress] = true
	}
	var selected []shipmentData
	for _, s := range ships {
		if s.Data.DeliveryStatus == "" || s.Data.DeliveryStatus == deliveryStatusToBeScheduled {
			selected = append(selected, s)
			locations[s.Data.PickupAddress] = true
			locations[s.Data.DeliveryAddress] = true
			if len(locations) >= maxNumLocs-1 {
				break
			}
		}
	}
	return selected
}

func riderToVehicle(r riderData) (v Vehicle, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("Error in rider %s: %s", r.Id, err)
		}
	}()
	lat, lon, err := GeocodeAddress(r.Data.StartAddress)
	if err != nil {
		return
	}
	start, err := unixTime(r.Data.EarliestStart)
	if err != nil {
		return
	}
	end, err := unixTime(r.Data.LatestEnd)
	if err != nil {
		return
	}
	return Vehicle{
		Id:            r.Id,
		Type:          r.Data.VehicleTypeId,
		StartAddress:  Address{r.Data.StartAddress, lat, lon},
		EarliestStart: start,
		LatestEnd:     end,
	}, nil
}

func dataToShipment(d shipmentData) (s Shipment, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("Error in shipment %s: %s", d.Id, err)
		}
	}()
	var (
		pickupAddr   = Address{Str: d.Data.PickupAddress}
		deliveryAddr = Address{Str: d.Data.DeliveryAddress}
	)
	pickupAddr.Lat, pickupAddr.Lon, err = GeocodeAddress(pickupAddr.Str)
	if err != nil {
		return
	}
	deliveryAddr.Lat, deliveryAddr.Lon, err = GeocodeAddress(deliveryAddr.Str)
	if err != nil {
		return
	}
	return Shipment{
		Id:       d.Id,
		Size:     [1]int{d.Data.Size},
		Pickup:   Delivery{pickupAddr, PickupPrepTime},
		Delivery: Delivery{deliveryAddr, DeliveryPrepTime},
	}, nil
}
