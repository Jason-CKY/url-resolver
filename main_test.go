package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestRouteNotFound(t *testing.T) {
	prefix := "/not-found"
	routingRules := ReadConfigRules(routingFilePath)
	weightedChoosers := GetRandomChooser(routingRules, false)
	_, found, err := ResolveUrl(prefix, weightedChoosers)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if found {
		t.Fatalf(`prefix %v not found in config file`, prefix)
	}
}

func TestEqualWeight(t *testing.T) {
	numIterations := 10000
	prefix := "/test1"
	routingRules := ReadConfigRules(routingFilePath)
	weightedChoosers := GetRandomChooser(routingRules, false)
	counts := make(map[string]int, len(weightedChoosers))
	for k := range weightedChoosers {
		counts[k] = 0
	}
	for i := 0; i < numIterations; i++ {
		route, found, err := ResolveUrl(prefix, weightedChoosers)
		if err != nil {
			t.Fatalf(err.Error())
		}
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
	routingRules := ReadConfigRules(routingFilePath)
	weightedChoosers := GetRandomChooser(routingRules, false)
	counts := make(map[string]int, len(weightedChoosers))
	for k := range weightedChoosers {
		counts[k] = 0
	}
	for i := 0; i < numIterations; i++ {
		route, found, err := ResolveUrl(prefix, weightedChoosers)
		if err != nil {
			t.Fatalf(err.Error())
		}
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

func TestValidateHealthCheck(t *testing.T) {
	routingRules := ReadConfigRules(routingFilePath)
	var noHealthCheckUpstreams []Upstream
	noHealthCheckUpstreams = append(noHealthCheckUpstreams, Upstream{Url: "http://test", HealthcheckEndpoint: nil})
	routingRules["error"] = RoutingRule{
		Upstreams: noHealthCheckUpstreams,
	}
	err := ValidateHealthCheck(routingRules)
	if err == nil {
		t.Fatalf("failed to raise error")
	}
}

func TestRoutingWithHealthCheck(t *testing.T) {
	routingRules := ReadConfigRules(routingFilePath)
	routingRules["/test1"].Upstreams[0].Healthy = true
	// Create a new Echo instance
	e := echo.New()
	// test1 should always return
	// Define the request
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/:prefix")
	c.SetParamNames("prefix")
	c.SetParamValues("test1")
	// Call the handler function
	routeHandler := func(c echo.Context) error {
		prefix := "/" + c.Param("prefix")
		weightedChoosers := GetRandomChooser(routingRules, true)
		route, found, err := ResolveUrl(prefix, weightedChoosers)
		if err != nil {
			return c.JSON(400, map[string]string{"detail": err.Error()})
		}
		if found {
			return c.JSON(200, map[string]string{"route": route})
		} else {
			return c.JSON(404, map[string]string{"detail": fmt.Sprintf("%s does not exist in the routing rules", prefix)})
		}
	}

	if assert.NoError(t, routeHandler(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		expectedResponse := map[string]string{"route": "http://10.20.10.10"}
		expectedResponseString, _ := json.Marshal(expectedResponse)
		assert.Equal(t, string(expectedResponseString), strings.TrimSuffix(rec.Body.String(), "\n"))
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetPath("/:prefix")
	c.SetParamNames("prefix")
	c.SetParamValues("test2")

	if assert.NoError(t, routeHandler(c)) {
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		expectedResponse := map[string]string{"detail": "no healthy upstream found"}
		expectedResponseString, _ := json.Marshal(expectedResponse)
		assert.Equal(t, string(expectedResponseString), strings.TrimSuffix(rec.Body.String(), "\n"))
	}
}
