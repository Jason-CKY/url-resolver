package main

import (
	"math"
	"testing"
)

func TestRouteNotFound(t *testing.T) {
	prefix := "/not-found"
	weightedChoosers := readConfig(routingFilePath)
	_, found := resolveUrl(prefix, weightedChoosers)
	if found {
		t.Fatalf(`prefix %v not found in config file`, prefix)
	}
}

func TestEqualWeight(t *testing.T) {
	numIterations := 10000
	prefix := "/test1"
	weightedChoosers := readConfig(routingFilePath)
	counts := make(map[string]int, len(weightedChoosers))
	for k := range weightedChoosers {
		counts[k] = 0
	}
	for i := 0; i < numIterations; i++ {
		route, found := resolveUrl(prefix, weightedChoosers)
		if !found {
			t.Fatalf(`prefix %v not found in config file`, prefix)
		}
		counts[route] += 1
	}
	ratio := float64(counts["http://10.20.10.10"]) / float64(counts["https://test.example.site"])
	if math.Abs(ratio-1.0) > 0.1 {
		t.Fatalf(`ratio %v off by more than 0.1`, ratio)
	}
}

func TestDifferentWeight(t *testing.T) {
	numIterations := 10000
	prefix := "/test2"
	weightedChoosers := readConfig(routingFilePath)
	counts := make(map[string]int, len(weightedChoosers))
	for k := range weightedChoosers {
		counts[k] = 0
	}
	for i := 0; i < numIterations; i++ {
		route, found := resolveUrl(prefix, weightedChoosers)
		if !found {
			t.Fatalf(`prefix %v not found in config file`, prefix)
		}
		counts[route] += 1
	}
	ratio := float64(counts["http://10.20.10.10"]) / float64(counts["https://test.example.site"])
	if math.Abs(ratio-1.0/20.0) > 0.1 {
		t.Fatalf(`ratio %v off by more than 0.1`, ratio)
	}
}
