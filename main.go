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
Usage: hop vehicles.csv shipments.csv your-graphhopper-api-key >result.csv`)
	}
	vehiclesName := args[0]
	shipmentsName := args[1]
	key := args[2]

	vehiclesTab := readTable(vehiclesName)
	shipmentsTab := readTable(shipmentsName)
	solutionTab, err := routeopt.SolveTables(vehiclesTab, shipmentsTab, key)
	if err != nil {
		log.Fatal(err)
	}

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
