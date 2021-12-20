package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("$PORT must be set!")
	}

	http.Handle("/", http.FileServer(http.Dir("./server/static")))
	http.HandleFunc("/solution.csv", csvEndpoint)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func setAllowOrigins(h http.Header) { h.Set("Access-Control-Allow-Origin", "*") }

func formatHourMin(unixTime int64) string {
	minutes := unixTime / 60
	min := minutes % 60
	hour := (minutes / 60) % 24
	format := "%d:%d"
	if min < 10 {
		format = "%d:0%d"
	}
	return fmt.Sprintf(format, hour, min)
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
