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
)

func main() {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("unable to load SDK config, %w", err)
	}

	d := discoverers.NewContinuousDiscoverer(
		discoverers.WithUpstreamDiscoverer(discoverers.NewAwsEc2Discoverer(cfg), time.Second*2),
		discoverers.WithUpstreamDiscoverer(discoverers.NewAwsEcsDiscoverer(cfg), time.Second*2),
		discoverers.WithUpstreamDiscoverer(discoverers.NewAwsRdsDiscoverer(cfg), time.Second*2),
	)

	results := make(chan *discovery.Result, 10)

	go d.Discover(ctx, results)

	for result := range results {
		for _, resource := range result.Resources {
			byt, err := json.Marshal(resource)
			if err != nil {
				fmt.Println(fmt.Sprintf("[ERROR] failed to json encode resource: %w", err))
			}
			fmt.Println(string(byt))
		}
	}
}
