package main

import (
	"encoding/csv"
	"flag"
	"log"
	"os"

	"github.com/robzan8/hop/routeopt"
)

var geocode bool

func main() {
	log.SetFlags(0)

	flag.BoolVar(&geocode, "geocode", false, "Tells if input csvs need to be geocoded")
	flag.Parse()
	args := flag.Args()
	if len(args) < 3 {
		log.Fatal(`Not enough input arguments provided.
Usage: hop [-geocode] vehicles.csv services.csv your-graphhopper-api-key`)
	}
	vehiclesName := args[0]
	servicesName := args[1]
	key := args[2]

	vehiclesTab := readTable(vehiclesName)
	servicesTab := readTable(servicesName)

	if geocode {
		routeopt.GeocodeTable(vehiclesTab, 2, key)
		routeopt.GeocodeTable(servicesTab, 1, key)
		vehiclesGeocoded := vehiclesName[0:len(vehiclesName)-4] + "_geocoded.csv"
		servicesGeocoded := servicesName[0:len(servicesName)-4] + "_geocoded.csv"
		writeTable(vehiclesTab, vehiclesGeocoded)
		writeTable(servicesTab, servicesGeocoded)
		return
	}

	vehicles := routeopt.ParseVehicles(vehiclesTab, key)
	services := routeopt.ParseServices(servicesTab, key)
	prob := routeopt.CreateProblem(vehicles, services)
	routeopt.Solve(prob, key)
}

func readTable(fileName string) [][]string {
	f, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	tab, err := r.ReadAll()
	if err != nil {
		log.Fatal(err)
	}
	return tab
}

func writeTable(tab [][]string, fileName string) {
	f, err := os.Create(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	err = w.WriteAll(tab)
	if err != nil {
		log.Fatal(err)
	}
}
