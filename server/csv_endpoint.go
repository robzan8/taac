package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

func csvEndpoint(w http.ResponseWriter, req *http.Request) {
	setAllowOrigins(w.Header())

	switch req.Method {
	case http.MethodOptions:
		// OK
	case http.MethodGet:
		fmt.Fprintln(w, "You should POST your shipments file here")
	case http.MethodPost:
		csvPost(w, req)
	default:
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Unsupported method %s", req.Method)
	}
}

func csvPost(w http.ResponseWriter, req *http.Request) {
	var err error // beware of shadowing
	defer func() {
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprintf(w, "%s", err)
		}
	}()

	routeoptKey, geocodeKey := req.FormValue("routeoptKey"), req.FormValue("geocodeKey")
	if routeoptKey == "" || geocodeKey == "" {
		err = fmt.Errorf("Some API key not provided")
		return
	}
	numVehicles, err := strconv.Atoi(req.FormValue("numVehicles"))
	if err != nil || numVehicles < 1 || numVehicles > 10 {
		err = fmt.Errorf("numVehicles must be an integer between 1 and 10")
		return
	}
	var startAddr Address
	startAddr.Str = req.FormValue("startAddress")
	startAddr.Lat, startAddr.Lon, err = GeocodeAddress(startAddr.Str, geocodeKey)
	if err != nil {
		return
	}
	startTime, err := unixTime(req.FormValue("startTime"))
	if err != nil {
		return
	}
	endTime, err := unixTime(req.FormValue("endTime"))
	if err != nil {
		return
	}
	var vehicles []Vehicle
	for i := 1; i <= numVehicles; i++ {
		vehicles = append(vehicles, Vehicle{
			Id:            fmt.Sprintf("rider%d", i),
			Type:          CargoBikeId,
			StartAddress:  startAddr,
			EarliestStart: startTime,
			LatestEnd:     endTime,
		})
	}

	f, _, err := req.FormFile("shipments")
	if err != nil {
		return
	}
	defer f.Close()
	ships, err := readCsvShipments(f, geocodeKey)
	if err != nil {
		return
	}

	problem := CreateProblem(vehicles, ships)
	solution, err := Solve(problem, routeoptKey)
	if err != nil {
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	err = writeCsvSolution(w, solution)
}

func readCsvShipments(in io.Reader, geocodeKey string) ([]Shipment, error) {
	var ships []Shipment
	r := csv.NewReader(in)
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		s, err := parseShipment(rec, geocodeKey)
		if err != nil {
			return nil, err
		}
		ships = append(ships, s)
	}
	return ships, nil
}

func parseShipment(rec []string, geocodeKey string) (Shipment, error) {
	if len(rec) != 3 {
		return Shipment{}, fmt.Errorf("Line in shipments csv must have 3 entries")
	}
	var s Shipment
	s.Id = rec[0]
	s.Size[0] = CargoBikeType.Capacity[0] / 20 // max 20 parcels per trip

	addr := rec[1]
	lat, lon, err := GeocodeAddress(addr, geocodeKey)
	if err != nil {
		return Shipment{}, err
	}
	s.Pickup.Address = Address{addr, lat, lon}
	s.Pickup.PrepTime = PickupPrepTime

	addr = rec[2]
	lat, lon, err = GeocodeAddress(addr, geocodeKey)
	if err != nil {
		return Shipment{}, err
	}
	s.Delivery.Address = Address{addr, lat, lon}
	s.Delivery.PrepTime = DeliveryPrepTime
	return s, nil
}

func writeCsvSolution(out io.Writer, s Solution) error {
	w := csv.NewWriter(out)
	err := w.Write([]string{
		"rider", "attività",
		"destinatario/contatti/note", "indirizzo", "orario",
	})
	if err != nil {
		return err
	}
	for _, route := range s.Solution.Routes {
		for _, act := range route.Activities {
			unixTime := act.ArrivalTime
			if unixTime == 0 {
				unixTime = act.EndTime
			}

			err = w.Write([]string{
				route.VehicleId, translateActType(act.Type),
				act.ShipmentId, act.Address.Str, formatHourMin(unixTime),
			})
			if err != nil {
				return err
			}
		}
	}
	for _, shipId := range s.Solution.Unassigned.Shipments {
		err = w.Write([]string{"", "non assegnato", shipId})
		if err != nil {
			return err
		}
	}
	return nil
}

func translateActType(t string) string {
	switch t {
	case "start":
		return "partenza"
	case "end":
		return "arrivo"
	case "pickupShipment":
		return "ritiro"
	case "deliverShipment":
		return "consegna"
	default:
		return t
	}
}