<https://taaaac.herokuapp.com/>

To run it locally:

    PORT=5000 GEOCODE_KEY=google-geocoding-key ROUTEOPT_KEY=graphhopper-key DINO_USER=username DINO_PASS=password go run server/main.go server/csv_endpoint.go server/schedule_endpoint.go server/geocoding.go server/routeopt.go
