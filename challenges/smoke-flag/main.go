package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	flag := os.Getenv("FLAG")
	if flag == "" {
		flag = os.Getenv("DYNAMIC_FLAG")
	}
	if flag == "" {
		flag = "FLAG{missing_dynamic_flag}"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/flag", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, flag)
	})

	addr := ":8080"
	log.Printf("smoke flag service listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
