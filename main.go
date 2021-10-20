package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var (
	auth   string
	client http.Client
)

const postUrl = "https://c2s.gnucoop.io/api/student"

func main() {
	flag.StringVar(&auth, "auth", "", "Authorization header")
	flag.Parse()
	args := flag.Args()
	log.SetFlags(0)

	if len(args) == 0 {
		log.Fatal(`No input files provided.
Usage: c2simport -auth="Bearer blabla" table.csv`)
	}

	for _, fileName := range args {
		c2sImportCsv(fileName)
	}
}

func c2sImportCsv(fileName string) {
	f, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		c2sImportRecord(rec)
	}
}

type Line struct {
	Name    string `json:"identifier"`
	Gender  string `json:"gender"`
	ClassId int    `json:"student_class_id"`
}

func c2sImportRecord(rec []string) {
	line := Line{
		Name:   rec[0],
		Gender: strings.ToLower(rec[1]),
	}
	var err error
	line.ClassId, err = strconv.Atoi(rec[2])
	if err != nil {
		log.Fatal("class id is not an integer")
	}
	body, err := json.Marshal(&line)
	if err != nil {
		log.Fatal(err)
	}

	req, err := http.NewRequest("POST", postUrl, bytes.NewReader(body))
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/json")
	if auth != "" {
		req.Header.Add("Authorization", auth)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		log.Fatalf("Unexpected response with code %d:\n%s", resp.StatusCode, body)
	}
}
