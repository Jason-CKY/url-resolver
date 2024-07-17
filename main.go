package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/mroth/weightedrand/v2"
	log "github.com/sirupsen/logrus"
)

var (
	// Config
	webPort                   int    = 8080
	routingFilePath           string = "routing.json"
	doHealthCheck             bool   = false
	healthCheckTimeoutSeconds int    = 5
)

type Upstream struct {
	Url                 string  `json:"url"`
	Weight              int     `json:"weight"`
	HealthcheckEndpoint *string `json:"healthcheck_endpoint"`
	Healthy             bool    `json:"ready"`
}

type RoutingRule struct {
	Upstreams []Upstream `json:"upstreams"`
}

func LookupEnvOrString(key string, defaultValue string) string {
	envVariable, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	return envVariable
}

func LookupEnvOrInt(key string, defaultValue int) int {
	envVariable, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	num, err := strconv.Atoi(envVariable)
	if err != nil {
		panic(err.Error())
	}
	return num
}

func LookupEnvOrBool(key string, defaultValue bool) bool {
	envVariable, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	return strings.ToLower(envVariable) == "true"
}

func ReadConfigRules(fpath string) map[string]RoutingRule {
	file, err := os.Open(fpath)
	if err != nil {
		log.Errorf("%v", err)
	}
	defer file.Close()
	byteValue, readErr := io.ReadAll(file)
	if readErr != nil {
		log.Errorf("%v", readErr)
	}
	var routingRules map[string]RoutingRule
	json.Unmarshal(byteValue, &routingRules)
	return routingRules
}

func GetRandomChooser(routingRules map[string]RoutingRule, doHealthCheck bool) map[string]*weightedrand.Chooser[string, int] {
	weightedChoosers := map[string]*weightedrand.Chooser[string, int]{}

	for prefix, routingRule := range routingRules {
		var weightedChoices []weightedrand.Choice[string, int]

		for i := 0; i < len(routingRule.Upstreams); i++ {
			upstream := routingRule.Upstreams[i]
			if (upstream.HealthcheckEndpoint != nil && upstream.Healthy) || !doHealthCheck {
				weightedChoices = append(weightedChoices, weightedrand.NewChoice(upstream.Url, upstream.Weight))
			}
		}
		weightedChoosers[prefix], _ = weightedrand.NewChooser(weightedChoices...)
	}

	return weightedChoosers
}

func ResolveUrl(prefix string, weightedChoosers map[string]*weightedrand.Chooser[string, int]) (route string, found bool, err error) {
	chooser, ok := weightedChoosers[prefix]
	if !ok {
		return "", ok, nil
	}
	if chooser == nil {
		return "", ok, errors.New("no healthy upstream found")
	}
	return chooser.Pick(), ok, nil
}

func HealthCheck(healthCheckEndpoint string) bool {
	req, httpErr := http.NewRequest(http.MethodGet, healthCheckEndpoint, nil)
	if httpErr != nil {
		return false
	}
	client := &http.Client{
		Timeout: time.Duration(healthCheckTimeoutSeconds) * time.Second,
	}
	res, httpErr := client.Do(req)
	if httpErr != nil {
		return false
	}
	return res.StatusCode == 200
}

func HealthCheckAll(routingRules map[string]RoutingRule) {
	for prefix := range routingRules {
		for i := 0; i < len(routingRules[prefix].Upstreams); i++ {
			if routingRules[prefix].Upstreams[i].HealthcheckEndpoint != nil {
				routingRules[prefix].Upstreams[i].Healthy = HealthCheck(*routingRules[prefix].Upstreams[i].HealthcheckEndpoint)
			}
		}
	}
}

func ValidateHealthCheck(routingRules map[string]RoutingRule) error {
	// if healthcheck is turned on, make sure config has health check endpoint for all route upstreams
	for prefix := range routingRules {
		for i := 0; i < len(routingRules[prefix].Upstreams); i++ {
			if routingRules[prefix].Upstreams[i].HealthcheckEndpoint == nil {
				return errors.New("healthcheck endpoint does not exist in config file")
			}
		}
	}
	return nil
}

func main() {
	// Load environment variables from .env file
	err := godotenv.Load()

	if err != nil {
		log.Infof("Error loading .env file: %v\nUsing environment variables instead...", err)
	}

	flag.StringVar(&routingFilePath, "fpath", LookupEnvOrString("CONFIG_FPATH", routingFilePath), "Path to routing json file")
	flag.IntVar(&webPort, "port", LookupEnvOrInt("PORT", webPort), "Port for echo web server")
	flag.BoolVar(&doHealthCheck, "healthcheck", LookupEnvOrBool("HEALTHCHECK", doHealthCheck), "Whether to do healthchecks for updating routing rules. Will not route to endpoints that are not ready")
	flag.IntVar(&healthCheckTimeoutSeconds, "healthcheck-timeout-seconds", LookupEnvOrInt("HEALTHCEHCK_TIMEOUT_SECONDS", healthCheckTimeoutSeconds), "Number of timeout seconds for healthcheck")
	flag.Parse()

	// setup logrus
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:          true,
		DisableLevelTruncation: true,
	})

	log.Infof("Reading routing file at %s", routingFilePath)
	routingRules := ReadConfigRules(routingFilePath)

	if doHealthCheck {
		err = ValidateHealthCheck(routingRules)
		if err != nil {
			log.Fatal(err.Error())
		}

		go func() {
			HealthCheckAll(routingRules)
			time.Sleep(5 * time.Second)
		}()
	}

	e := echo.New()
	e.GET("/health", func(c echo.Context) error {
		return c.String(200, "Healthy")
	})
	e.GET("/:prefix", func(c echo.Context) error {
		prefix := "/" + c.Param("prefix")
		weightedChoosers := GetRandomChooser(routingRules, doHealthCheck)
		route, found, err := ResolveUrl(prefix, weightedChoosers)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"detail": err.Error()})
		}
		if found {
			return c.JSON(http.StatusOK, map[string]string{"route": route})
		} else {
			return c.JSON(http.StatusNotFound, map[string]string{"detail": fmt.Sprintf("%s does not exist in the routing rules", prefix)})
		}

	})
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%v", webPort)))

}
