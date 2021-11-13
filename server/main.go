package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/robzan8/hop/geocoding"
	"github.com/robzan8/hop/routeopt"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("$PORT must be set!")
	}

	http.Handle("/", http.FileServer(http.Dir("./server/static")))
	http.HandleFunc("/solution.csv", solve)
	http.HandleFunc("/geocoded.csv", geocode)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func setAllowOrigins(h http.Header) { h.Set("Access-Control-Allow-Origin", "*") }

func solve(w http.ResponseWriter, r *http.Request) {
	setAllowOrigins(w.Header())

	switch r.Method {
	case http.MethodOptions:
		// OK
	case http.MethodGet:
		fmt.Fprintln(w, "You should POST your vehicles and services files here")
	case http.MethodPost:
		solvePost(w, r)
	default:
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Unsupported method %s", r.Method)
	}
}

func solvePost(w http.ResponseWriter, r *http.Request) {
	key := r.FormValue("key")
	if key == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "GraphHopper API key not provided")
		return
	}
	vehiclesTab, err := readTable(r, "vehicles")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error reading vehicles POST file: %s", err)
		return
	}
	servicesTab, err := readTable(r, "services")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error reading services POST file: %s", err)
		return
	}

	solutionTab, err := routeopt.SolveTables(vehiclesTab, servicesTab, key)
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, "Error solving routeopt problem: %s", err)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	err = csv.NewWriter(w).WriteAll(solutionTab)
	if err != nil {
		log.Printf("Error writing csv response: %s", err)
	}
}

func geocode(w http.ResponseWriter, r *http.Request) {
	setAllowOrigins(w.Header())

	switch r.Method {
	case http.MethodOptions:
		// OK
	case http.MethodGet:
		fmt.Fprintln(w, "You should POST your vehicles or services file here")
	case http.MethodPost:
		geocodePost(w, r)
	default:
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Unsupported method %s", r.Method)
	}
}

func geocodePost(w http.ResponseWriter, r *http.Request) {
	key := r.FormValue("key")
	if key == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Google geocoding API key not provided")
		return
	}
	tab, err := readTable(r, "table")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error reading csv POST file: %s", err)
		return
	}

	err = geocoding.GeocodeTable(tab, key, nil)
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, "Error geocoding table: %s", err)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	err = csv.NewWriter(w).WriteAll(tab)
	if err != nil {
		log.Printf("Error writing csv response: %s", err)
	}
}

func readTable(r *http.Request, fileName string) ([][]string, error) {
	f, _, err := r.FormFile(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return csv.NewReader(f).ReadAll()
}
