package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/fornellas/brother_exporter/brother"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	address = kingpin.Flag("server", "server address").Default(":8035").String()
)

func metricsHandler(w http.ResponseWriter, req *http.Request) {
	log.Printf("< %s GET /probe", req.RemoteAddr)

	brotherUrls, ok := req.URL.Query()["address"]
	if !ok {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Missing 'address' query parameter")
		return
	}
	if len(brotherUrls) != 1 {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Multiple 'address' query parameter")
		return
	}
	brotherUrl := brotherUrls[0]

	brotherReq, err := http.NewRequest("GET", brotherUrl, nil)
	if err != nil {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Bad 'address': %s", err)
		return
	}

	brotherRes, err := http.DefaultClient.Do(brotherReq)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "GET %s failed: %s", brotherUrl, err)
		return
	}
	if brotherRes.StatusCode != 200 {
		w.WriteHeader(500)
		fmt.Fprintf(w, "GET %s failed returned %d: %s", brotherUrl, brotherRes.StatusCode, err)
		return
	}
	contentType := brotherRes.Header.Get("Content-Type")
	expectedContentType := "text/comma-separated-values"
	if contentType != expectedContentType {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Expected GET %s to return Content-Type %s, returned %s", brotherUrl, expectedContentType, contentType)
		return
	}

	timeSeriesGroup, err := brother.ReadMaintenanceInfo(brotherRes.Body)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Failed to parse: %s", err)
		return
	}

	fmt.Fprintf(w, "%s", timeSeriesGroup)
	log.Printf("> %s GET /probe 200", req.RemoteAddr)
}

func main() {
	kingpin.Parse()

	http.HandleFunc("/probe", metricsHandler)

	log.Printf("Listening at %s", *address)
	if err := http.ListenAndServe(*address, nil); err != http.ErrServerClosed {
		log.Fatalf("Failed to start server: %s", err.Error())
	}
}
