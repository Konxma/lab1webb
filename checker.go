package main

import (
	"net/http"
	"time"
)

type Result struct {
	URL        string
	StatusCode int
	Duration   time.Duration
	Err        error
}

type Checker interface {
	Check(url string) Result
}

type HTTPChecker struct {
	client *http.Client
}

func NewHTTPChecker(timeout time.Duration) *HTTPChecker {
	return &HTTPChecker{client: &http.Client{
		Transport: http.DefaultTransport.(*http.Transport).Clone(),
		Timeout:   timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
}

func (c *HTTPChecker) Check(url string) Result {
	start := time.Now()
	response, err := c.client.Get(url)
	result := Result{URL: url, Duration: time.Since(start), Err: err}
	if err == nil {
		result.StatusCode = response.StatusCode
		_ = response.Body.Close()
	}
	return result
}

func (c *HTTPChecker) Close() {
	c.client.CloseIdleConnections()
}

type Stats struct {
	Total         int
	Healthy       int
	OtherStatuses int
	RequestErrors int
}

func (s *Stats) Record(result Result) {
	s.Total++
	switch {
	case result.Err != nil:
		s.RequestErrors++
	case result.StatusCode >= 200 && result.StatusCode < 300:
		s.Healthy++
	default:
		s.OtherStatuses++
	}
}
