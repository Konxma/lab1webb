package main

import (
	"net/http"
	"net/http/httptest"
	"time"
)

func startDemo() (*httptest.Server, []string) {
	mux := http.NewServeMux()
	fixtures := []struct {
		path   string
		status int
		delay  time.Duration
	}{
		{"/ok", http.StatusOK, 140 * time.Millisecond},
		{"/created", http.StatusCreated, 180 * time.Millisecond},
		{"/missing", http.StatusNotFound, 115 * time.Millisecond},
		{"/unavailable", http.StatusServiceUnavailable, 200 * time.Millisecond},
	}
	for _, fixture := range fixtures {
		mux.HandleFunc(fixture.path, func(w http.ResponseWriter, r *http.Request) {
			timer := time.NewTimer(fixture.delay)
			defer timer.Stop()
			select {
			case <-timer.C:
				w.WriteHeader(fixture.status)
			case <-r.Context().Done():
			}
		})
	}
	mux.HandleFunc("/timeout", func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	demo := httptest.NewServer(mux)
	urls := make([]string, 0, len(fixtures)+1)
	for _, fixture := range fixtures {
		urls = append(urls, demo.URL+fixture.path)
	}
	return demo, append(urls, demo.URL+"/timeout")
}
