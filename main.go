package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"development.thatwebsite.xyz/gokrazy/acron/config"
	yaml "gopkg.in/yaml.v3"
)

var (
	configFile = flag.String("config", "acron.yml", "Config File")
)

func main() {
	flag.Parse()

	log.Printf("Using configuration %s", *configFile)

	f, err := os.Open(*configFile)
	if err != nil {
		log.Fatal(err)
	}

	cfg := &config.Config{}
	if strings.HasSuffix(*configFile, ".yml") || strings.HasSuffix(*configFile, ".yaml") {
		if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
			log.Fatal(err)
		}

	} else {
		if err := json.NewDecoder(f).Decode(&cfg); err != nil {
			log.Fatal(err)
		}
	}
	if err := cfg.Init(); err != nil {
		log.Fatal(err)
	}

	var tickerDuration time.Duration

	if tickerDuration, err = time.ParseDuration(cfg.TickerDuration); err != nil {
		log.Println("invalid ticker duration, using default, error:", err)
		tickerDuration = time.Minute
	}

	t := time.NewTicker(tickerDuration)
	tm := time.Now()
	ctx := context.WithValue(context.Background(), config.FirstKey, true)
	for {
		if err := cfg.Check(ctx); err != nil {
			log.Printf("Failed to check on %s, error: %v", tm.Format(time.RFC3339), err)
			continue
		}
		log.Printf("Successful check on %s", tm.Format(time.RFC3339))
		ctx = context.WithValue(ctx, config.FirstKey, false)
		tm = <-t.C
	}
}
