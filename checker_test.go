package main

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPChecker(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/fail", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(503) })
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ok", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/timeout", func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	server := httptest.NewServer(mux)
	defer server.Close()
	checker := NewHTTPChecker(time.Second)
	defer checker.Close()
	for _, item := range []struct {
		path   string
		status int
	}{{"/ok", 200}, {"/absent", 404}, {"/fail", 503}, {"/redirect", 307}} {
		url := server.URL + item.path
		result := checker.Check(url)
		if result.URL != url || result.StatusCode != item.status || result.Duration < 0 || result.Err != nil {
			t.Fatalf("Unexpected HTTP result: %+v", result)
		}
	}
	if result := checker.Check("not a URL"); result.Err == nil || result.StatusCode != 0 {
		t.Fatalf("Unexpected malformed URL result: %+v", result)
	}
	withTimeout := NewHTTPChecker(50 * time.Millisecond)
	defer withTimeout.Close()
	result := withTimeout.Check(server.URL + "/timeout")
	var err net.Error
	if !errors.As(result.Err, &err) || !err.Timeout() || result.StatusCode != 0 {
		t.Fatalf("Expected a timeout: %+v", result)
	}
}

type fakeChecker map[string]Result

func (f fakeChecker) Check(target string) Result {
	result := f[target]
	result.URL = target
	return result
}

func TestModesCountEveryRequest(t *testing.T) {
	checker := fakeChecker{
		"ok": {StatusCode: 200}, "created": {StatusCode: 201}, "missing": {StatusCode: 404},
		"unavailable": {StatusCode: 503}, "broken": {Err: errors.New("unreachable")},
	}
	var urls []string
	for i := 0; i < 20; i++ {
		urls = append(urls, "ok", "created", "missing", "unavailable", "broken")
	}
	want := Stats{Total: 100, Healthy: 40, OtherStatuses: 40, RequestErrors: 20}
	for name, run := range map[string]func(Checker, []string) ([]Result, Stats){
		"sequential": sequential, "mutex": concurrentMutex,
	} {
		results, stats := run(checker, urls)
		if stats != want || len(results) != len(urls) {
			t.Fatalf("%s: unexpected results or totals: %v (%d results)", name, stats, len(results))
		}
		for i, url := range urls {
			if results[i].URL != url || results[i].StatusCode != checker[url].StatusCode {
				t.Fatalf("%s: entry %d = %+v", name, i, results[i])
			}
		}
	}
	var stats Stats
	seen := make(map[string]int)
	for result := range concurrentChannel(checker, urls) {
		stats.Record(result)
		seen[result.URL]++
	}
	if stats != want || len(seen) != len(checker) {
		t.Fatalf("channel: unexpected totals: %v (%v)", stats, seen)
	}
	for url := range checker {
		if seen[url] != 20 {
			t.Fatalf("channel: %s checked %d times", url, seen[url])
		}
	}
	if _, ok := <-concurrentChannel(checker, nil); ok {
		t.Fatal("Channel for an empty list must close")
	}
}
