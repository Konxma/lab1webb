package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

var modeLabels = map[string]string{
	"sequential": "Послідовна перевірка",
	"mutex":      "Конкурентна перевірка (WaitGroup + Mutex)",
	"channel":    "Конкурентна перевірка (WaitGroup + chan)",
}

func main() {
	mode := flag.String("mode", "all", "режим: sequential, mutex, channel або all")
	timeout := flag.Duration("timeout", 0, "таймаут одного запиту (за замовчуванням 3s, у демо 350ms)")
	demo := flag.Bool("demo", false, "перевірити локальні демонстраційні URL")
	flag.Parse()
	urls := flag.Args()
	modes := []string{*mode}
	if *mode == "all" {
		modes = []string{"sequential", "mutex", "channel"}
	} else if _, ok := modeLabels[*mode]; !ok {
		fmt.Fprintln(os.Stderr, "Невідомий режим. Доступні: sequential, mutex, channel, all.")
		os.Exit(2)
	}
	if *timeout < 0 {
		fmt.Fprintln(os.Stderr, "Таймаут має бути невід’ємним.")
		os.Exit(2)
	}
	if *demo && len(urls) > 0 {
		fmt.Fprintln(os.Stderr, "Режим -demo використовує власні URL.")
		os.Exit(2)
	}
	if *demo {
		server, targets := startDemo()
		defer server.Close()
		urls = targets
		if *timeout == 0 {
			*timeout = 350 * time.Millisecond
		}
	} else if len(urls) == 0 {
		urls = []string{
			"https://go.dev", "https://github.com", "https://www.iana.org",
			"https://httpbin.org/status/404", "https://health-check-does-not-exist.invalid",
		}
	}
	if *timeout == 0 {
		*timeout = 3 * time.Second
	}

	fmt.Println("HTTP URL Health Checker")
	fmt.Printf("Адрес: %d | Таймаут одного запиту: %s\n", len(urls), *timeout)
	elapsed := make(map[string]time.Duration, len(modes))
	for _, mode := range modes {
		elapsed[mode] = runMode(mode, urls, *timeout)
	}
	if baseline, parallel := elapsed["sequential"], elapsed["channel"]; baseline > 0 && parallel > 0 {
		fmt.Printf("\nПослідовно: %s | Канали: %s | Прискорення: %.2f×\n",
			baseline.Round(time.Microsecond), parallel.Round(time.Microsecond), float64(baseline)/float64(parallel))
	}
}

func runMode(mode string, urls []string, timeout time.Duration) time.Duration {
	checker := NewHTTPChecker(timeout)
	defer checker.Close()
	var results []Result
	var stats Stats
	start := time.Now()
	switch mode {
	case "sequential":
		results, stats = sequential(checker, urls)
	case "mutex":
		results, stats = concurrentMutex(checker, urls)
	case "channel":
		results = make([]Result, 0, len(urls))
		for result := range concurrentChannel(checker, urls) {
			results = append(results, result)
			stats.Record(result)
		}
	}
	elapsed := time.Since(start)

	fmt.Printf("\n%s\n", modeLabels[mode])
	for _, result := range results {
		if result.Err != nil {
			fmt.Printf("  %s → помилка (%s)\n    %v\n", result.URL, result.Duration.Round(time.Microsecond), result.Err)
			continue
		}
		fmt.Printf("  %s → %d %s (%s)\n", result.URL, result.StatusCode, http.StatusText(result.StatusCode), result.Duration.Round(time.Microsecond))
	}
	fmt.Printf("  Підсумок | Усього: %d | HTTP-відповіді: %d | 2xx: %d | Інші HTTP: %d | Помилки: %d | Час: %s\n",
		stats.Total, stats.Healthy+stats.OtherStatuses, stats.Healthy, stats.OtherStatuses, stats.RequestErrors, elapsed.Round(time.Microsecond))
	return elapsed
}
