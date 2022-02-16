package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

var (
	port        = os.Getenv("PORT")
	geocodeKey  = os.Getenv("GEOCODE_KEY")
	routeoptKey = os.Getenv("ROUTEOPT_KEY")
	dinoUser    = os.Getenv("DINO_USER")
	dinoPass    = os.Getenv("DINO_PASS")
)

func main() {
	if port == "" || geocodeKey == "" || routeoptKey == "" ||
		dinoUser == "" || dinoPass == "" {
		log.Fatal("Some environment variable not set")
	}

	rand.Seed(time.Now().UnixNano())

	http.Handle("/", http.FileServer(http.Dir("./server/static")))
	http.HandleFunc("/solution.csv", csvEndpoint)
	http.HandleFunc("/schedule.txt", scheduleEndpoint)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func setAllowOrigins(h http.Header) { h.Set("Access-Control-Allow-Origin", "*") }

func formatHourMin(unixTime int64) string {
	minutes := unixTime / 60
	min := minutes % 60
	hour := (minutes / 60) % 24
	return fmt.Sprintf("%02d:%02d", hour, min)
}

// hourMin is in the format "23:59"
func unixTime(hourMin string) (int64, error) {
	var hour, min int64
	_, err := fmt.Sscanf(hourMin, "%d:%d", &hour, &min)
	if err != nil || hour < 0 || hour > 23 || min < 0 || min > 59 {
		return 0, fmt.Errorf("Wrongly formatted time %q", hourMin)
	}
	return (hour*60 + min) * 60, nil
}
