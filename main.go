package main

import (
	"encoding/csv"
	"log"
	"os"

	"github.com/robzan8/hop/routeopt"
)

func main() {
	log.SetFlags(0)

	args := os.Args[1:]
	if len(args) < 3 {
		log.Fatal(`Not enough input arguments provided.
Usage: hop vehicles.csv services.csv your-graphhopper-api-key >result.csv`)
	}
	vehiclesName := args[0]
	servicesName := args[1]
	key := args[2]

	vehiclesTab := readTable(vehiclesName)
	servicesTab := readTable(servicesName)
	vehicles, err := routeopt.ParseVehicles(vehiclesTab)
	if err != nil {
		log.Fatal(err)
	}
	services, err := routeopt.ParseServices(servicesTab)
	if err != nil {
		log.Fatal(err)
	}
	problem := routeopt.CreateProblem(vehicles, services)
	solution, err := routeopt.Solve(problem, key)
	if err != nil {
		log.Fatal(err)
	}
	solutionTab := routeopt.SolutionToTab(solution)

	w := csv.NewWriter(os.Stdout)
	err = w.WriteAll(solutionTab)
	if err != nil {
		log.Fatal(err)
	}
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
