package main

import (
	"log"
	"net/http"
)

func main() {
	log.Println("Starting the Bosh System Metrics Forwarder on 8080")
	http.ListenAndServe(":8080", nil)
}
