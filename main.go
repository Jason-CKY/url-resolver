package main

import (
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

var (
	// Config
	routingFilePath string = "routing.json"
	routingRules    map[string]RoutingRule
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

func readConfig(fpath string) map[string]RoutingRule {
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

func resolveUrl(c *gin.Context) {
	prefix := "/" + c.Param("prefix")

	upstreams, ok := routingRules[prefix]
	if ok {
		log.Info(upstreams)
	} else {
		log.Errorf("Prefix not found in routing rules")
	}
	c.JSON(200, gin.H{"route": prefix})
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
	routingRules = readConfig(routingFilePath)

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
