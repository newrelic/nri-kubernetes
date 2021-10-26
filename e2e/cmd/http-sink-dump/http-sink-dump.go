package main

import (
	"fmt"
	"io"
	"net/http"
)

const defaultURIData = "/v1/data"

func main() {
	dataHandler := func(w http.ResponseWriter, req *http.Request) {
		b, _ := io.ReadAll(req.Body)
		req.Body.Close()
		fmt.Printf(string(b))
		w.WriteHeader(204)
	}

	http.HandleFunc(defaultURIData, dataHandler)

	err := http.ListenAndServe(":8001", nil)
	if err != nil {
		panic(err)
	}
}
