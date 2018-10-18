package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

type request struct {
	Inputs          interface{} `json:"inputs"`
	ReferenceInputs interface{} `json:"referenceInputs"`
	Parameters      interface{} `json:"parameters"`
	Outputs         interface{} `json:"outputs"`
	Labels          [][]string  `json:"labels"`
	Returns         interface{} `json:"returns"`
}

func main() {
	http.HandleFunc("/dummy/dummy_ok", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("error: unable to read body [err=%v]", err)
		} else {
			req := request{}
			if err := json.Unmarshal(body, &req); err != nil {
				log.Printf("error: unable to unmarshal body [err=%v]", err)
			} else {
				log.Printf("inputs:  %v", req.Inputs)
				log.Printf("outputs: %v", req.Outputs)
			}
		}

		w.Header().Add("Content-Type", "application/json")
		fmt.Fprintf(w, "{\"success\": true}")
	})
	http.HandleFunc("/dummy/dummy_ko", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		fmt.Fprintf(w, "{\"success\": false}")
	})
	http.HandleFunc("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		fmt.Fprintf(w, "{\"ok\": true}")
	})

	log.Printf("starting http server on 0.0.0.0:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
