package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/borderzero/discovery"
	"github.com/borderzero/discovery/discoverers"
	"github.com/borderzero/discovery/engines"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*30)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("unable to load SDK config, %w", err)
	}

	engine := engines.NewContinuousEngine(
		engines.WithDiscoverer(
			discoverers.NewAwsEc2Discoverer(cfg),
			engines.WithInitialInterval(time.Second*2),
		),
		engines.WithDiscoverer(
			discoverers.NewAwsEcsDiscoverer(cfg),
			engines.WithInitialInterval(time.Second*2),
		),
		engines.WithDiscoverer(
			discoverers.NewAwsRdsDiscoverer(cfg),
			engines.WithInitialInterval(time.Second*2),
		),
	)

	results := make(chan *discovery.Result, 10)

	go engine.Run(ctx, results)

	for result := range results {
		byt, err := json.Marshal(result)
		if err != nil {
			fmt.Println(fmt.Sprintf("[ERROR] failed to json encode result: %w", err))
		}
		fmt.Println(string(byt))
	}
}
