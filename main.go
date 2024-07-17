package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/mroth/weightedrand/v2"
	log "github.com/sirupsen/logrus"
)

var (
	// Config
	webPort         int    = 8080
	routingFilePath string = "routing.json"
	doHealthCheck   bool   = false
)

type RoutingRule map[string][]Upstream

type Upstream struct {
	Url    string `json:"url"`
	Weight int    `json:"weight"`
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

func readConfig(fpath string) map[string]*weightedrand.Chooser[string, int] {
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

	weightedChoosers := map[string]*weightedrand.Chooser[string, int]{}

	for prefix, routingRule := range routingRules {
		var weightedChoices []weightedrand.Choice[string, int]

		for i := 0; i < len(routingRule["upstreams"]); i++ {
			upstream := routingRule["upstreams"][i]
			weightedChoices = append(weightedChoices, weightedrand.NewChoice(upstream.Url, upstream.Weight))
		}
		weightedChoosers[prefix], _ = weightedrand.NewChooser(weightedChoices...)
	}

	return weightedChoosers
}

func resolveUrl(prefix string, weightedChoosers map[string]*weightedrand.Chooser[string, int]) (route string, found bool) {
	chooser, ok := weightedChoosers[prefix]
	if !ok {
		return "", ok
	}
	return chooser.Pick(), ok
}

func main() {
	flag.StringVar(&routingFilePath, "fpath", LookupEnvOrString("CONFIG_FPATH", routingFilePath), "Path to routing json file")
	flag.IntVar(&webPort, "port", LookupEnvOrInt("PORT", webPort), "Port for echo web server")
	flag.BoolVar(&doHealthCheck, "healthcheck", LookupEnvOrBool("HEALTHCHECK", doHealthCheck), "Whether to do healthchecks for updating routing rules. Will not route to endpoints that are not ready")
	flag.Parse()

	// setup logrus
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:          true,
		DisableLevelTruncation: true,
	})

	log.Infof("Reading routing file at %s", routingFilePath)
	weightedChoosers := readConfig(routingFilePath)

	e := echo.New()
	e.GET("/:prefix", func(c echo.Context) error {
		prefix := "/" + c.Param("prefix")
		route, found := resolveUrl(prefix, weightedChoosers)
		if found {
			return c.JSON(200, map[string]string{"route": route})
		} else {
			log.Errorf("Prefix not found in routing rules")
			return c.JSON(404, map[string]string{"detail": fmt.Sprintf("%s does not exist in the routing rules", prefix)})
		}

	})
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%v", webPort)))

}
