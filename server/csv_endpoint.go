package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
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
	if req.FormValue("password") != password {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, "Wrong password")
		return
	}

	var err error // beware of shadowing
	defer func() {
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprintf(w, "%s", err)
		}
	}()

	schedDate := req.FormValue("date")
	if !dateRegex.MatchString(schedDate) {
		err = fmt.Errorf("date must be in the format 2022-12-31")
		return
	}
	ridersList := req.FormValue("riders")
	if ridersList == "" {
		err = fmt.Errorf("empty riders list")
		return
	}
	parcelsPerBike, err := strconv.Atoi(req.FormValue("parcelsPerBike"))
	if err != nil || parcelsPerBike < 1 || parcelsPerBike > 100 {
		err = fmt.Errorf("parcelsPerBike must be an integer between 1 and 100")
		return
	}
	var startAddr Address
	startAddr.Str = req.FormValue("startAddress")
	startAddr.Lat, startAddr.Lon, err = GeocodeAddress(startAddr.Str)
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
	for _, riderName := range strings.Split(ridersList, ",") {
		vehicles = append(vehicles, Vehicle{
			Id:            riderName,
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
	shipSize := CargoBikeType.Capacity[0] / parcelsPerBike
	ships, err := readCsvShipments(f, geocodeKey, shipSize)
	if err != nil {
		return
	}

	problem := CreateProblem(vehicles, ships)
	solution, err := Solve(problem)
	if err != nil {
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	err = writeCsvSolution(w, solution)
}

func readCsvShipments(in io.Reader, geocodeKey string, shipSize int) ([]Shipment, error) {
	var ships []Shipment
	r := csv.NewReader(in)
	_, err := r.Read() // read away the header
	if err == io.EOF {
		return nil, fmt.Errorf("Empty shipments file")
	}
	if err != nil {
		return nil, err
	}
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		s, err := parseShipment(rec, geocodeKey, shipSize)
		if err != nil {
			return nil, err
		}
		ships = append(ships, s)
	}
	return ships, nil
}

func parseShipment(rec []string, geocodeKey string, shipSize int) (Shipment, error) {
	if len(rec) != 3 {
		return Shipment{}, fmt.Errorf("Line in shipments csv must have 3 entries")
	}
	var s Shipment
	s.Id = rec[0]
	s.Size[0] = shipSize

	addr := rec[1]
	lat, lon, err := GeocodeAddress(addr)
	if err != nil {
		return Shipment{}, err
	}
	s.Pickup.Address = Address{addr, lat, lon}
	s.Pickup.PrepTime = PickupPrepTime

	addr = rec[2]
	lat, lon, err = GeocodeAddress(addr)
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
	w.Flush()
	return w.Error()
}

func translateActType(t string) string {
	switch t {
	case ActivityTypeStart:
		return "partenza"
	case ActivityTypeEnd:
		return "arrivo"
	case ActivityTypePickup:
		return "ritiro"
	case ActivityTypeDeliver:
		return "consegna"
	default:
		return t
	}
}
