package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mroth/weightedrand/v2"
	log "github.com/sirupsen/logrus"
)

var (
	// Config
	routingFilePath  string = "routing.json"
	weightedChoosers map[string]*weightedrand.Chooser[string, int]
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

func resolveUrl(c *gin.Context) {
	prefix := "/" + c.Param("prefix")

	chooser, ok := weightedChoosers[prefix]
	if ok {
		c.JSON(200, gin.H{"route": chooser.Pick()})
	} else {
		log.Errorf("Prefix not found in routing rules")
		c.JSON(404, gin.H{"detail": fmt.Sprintf("%s does not exist in the routing rules", prefix)})
	}
}

func main() {
	flag.StringVar(&routingFilePath, "fpath", LookupEnvOrString("CONFIG_FPATH", routingFilePath), "Path to routing json file")

	flag.Parse()

	// setup logrus
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:          true,
		DisableLevelTruncation: true,
	})

	log.Infof("Reading routing file at %s", routingFilePath)
	weightedChoosers = readConfig(routingFilePath)

	router := gin.Default()
	router.GET("/:prefix", resolveUrl)
	s := &http.Server{
		Addr:           ":8080",
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	s.ListenAndServe()
}
