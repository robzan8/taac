package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"sort"
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
			Id:            strings.TrimSpace(riderName),
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
	shipData, err := readCsvShipments(f, shipSize)
	if err != nil {
		return
	}
	var ships []Shipment
	for _, d := range shipData {
		var s Shipment
		s, err = dataToShipment(d)
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

	writeSolutionIntoShipments(shipData, solution, schedDate)
	sort.SliceStable(shipData, func(i, j int) bool {
		return shipData[i].Data.DeliveryTime < shipData[j].Data.DeliveryTime
	})
	sort.SliceStable(shipData, func(i, j int) bool {
		// We want unassigned shipments at the end.
		if shipData[i].Data.RiderName == "" {
			return false
		}
		if shipData[j].Data.RiderName == "" {
			return true
		}
		return shipData[i].Data.RiderName < shipData[j].Data.RiderName
	})

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	err = writeCsvShipments(w, shipData)
}

func readCsvShipments(in io.Reader, shipSize int) ([]shipmentData, error) {
	var ships []shipmentData
	r := csv.NewReader(in)
	_, err := r.Read() // read away the header
	if err == io.EOF {
		return nil, fmt.Errorf("Empty shipments file")
	}
	if err != nil {
		return nil, err
	}
	for i := 1; true; i++ {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if len(rec) != 3 {
			return nil, fmt.Errorf("Line in shipments csv must have 3 entries")
		}
		var s shipmentData
		s.Id = strconv.Itoa(i)
		s.Data.Size = shipSize
		s.Data.PickupAddress = rec[1]
		s.Data.DeliveryAddress = rec[2]
		s.Data.Notes = rec[0]
		ships = append(ships, s)
	}
	return ships, nil
}

func writeCsvShipments(out io.Writer, ships []shipmentData) error {
	w := csv.NewWriter(out)
	err := w.Write([]string{
		"rider", "destinatario/contatti/note", "indirizzo di ritiro",
		"indirizzo di consegna", "giorno", "orario di ritiro", "orario di consegna",
	})
	if err != nil {
		return err
	}
	for _, s := range ships {
		d := s.Data
		err = w.Write([]string{
			d.RiderName, d.Notes, d.PickupAddress, d.DeliveryAddress,
			d.ShipmentDay, d.PickupTime, d.DeliveryTime,
		})
		if err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
