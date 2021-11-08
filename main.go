package main

import (
	"encoding/csv"
	"log"
	"os"
	"time"

	"github.com/robzan8/hop/routeopt"
)

func main() {
	log.SetFlags(0)
	now := time.Now()

	args := os.Args[1:]
	if len(args) < 3 {
		log.Fatal(`Not enough input arguments provided.
Usage: hop vehicles.csv services.csv your-graphhopper-api-key`)
	}
	vehiclesName := args[0]
	servicesName := args[1]
	key := args[2]

	vehiclesTab := readTable(vehiclesName)
	servicesTab := readTable(servicesName)
	vehicles := routeopt.ParseVehicles(vehiclesTab, now)
	services := routeopt.ParseServices(servicesTab, now)
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
