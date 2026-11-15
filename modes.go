package main

import "sync"

func sequential(checker Checker, urls []string) ([]Result, Stats) {
	results := make([]Result, 0, len(urls))
	var stats Stats
	for _, url := range urls {
		result := checker.Check(url)
		results = append(results, result)
		stats.Record(result)
	}
	return results, stats
}

func concurrentMutex(checker Checker, urls []string) ([]Result, Stats) {
	results := make([]Result, len(urls))
	var stats Stats
	var mu sync.Mutex
	var workers sync.WaitGroup
	for i, url := range urls {
		workers.Add(1)
		go func(index int, target string) {
			defer workers.Done()
			result := checker.Check(target)
			results[index] = result
			mu.Lock()
			stats.Record(result)
			mu.Unlock()
		}(i, url)
	}
	workers.Wait()
	return results, stats
}

func concurrentChannel(checker Checker, urls []string) <-chan Result {
	results := make(chan Result)
	var workers sync.WaitGroup
	for _, url := range urls {
		workers.Add(1)
		go func(target string) {
			defer workers.Done()
			results <- checker.Check(target)
		}(url)
	}
	go func() {
		workers.Wait()
		close(results)
	}()
	return results
}
