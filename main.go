package main

import (
	"log"
	"os"

	"github.com/robzan8/hop/routeopt"
)

func main() {
	log.SetFlags(0)

	args := os.Args[1:]
	if len(args) < 3 {
		log.Fatal(`Not enough input arguments provided.
Usage: hop vehicles.csv services.csv your-graphhopper-api-key`)
	}
	key := args[2]

	vehicles := routeopt.ReadVehicles(args[0], key)
	services := routeopt.ReadServices(args[1], key)
	prob := routeopt.CreateProblem(vehicles, services)
	routeopt.Solve(prob, key)
}
