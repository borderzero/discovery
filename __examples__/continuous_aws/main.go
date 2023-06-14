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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*12)
	defer cancel()

	awsConfigLoadCtx, awsConfigLoadCtxCancel := context.WithTimeout(ctx, time.Second)
	defer awsConfigLoadCtxCancel()

	cfg, err := config.LoadDefaultConfig(awsConfigLoadCtx)
	if err != nil {
		log.Fatalf("unable to load SDK config, %w", err)
	}

	d := discoverers.NewContinuousDiscoverer(
		discoverers.WithUpstreamDiscoverer(discoverers.NewAwsEc2Discoverer(cfg), time.Second*5),
		discoverers.WithUpstreamDiscoverer(discoverers.NewAwsEcsDiscoverer(cfg), time.Second*5),
		discoverers.WithUpstreamDiscoverer(discoverers.NewAwsRdsDiscoverer(cfg), time.Second*5),
	)

	results := make(chan *discovery.Result, 10)

	go d.Discover(ctx, results)

	for result := range results {
		byt, _ := json.Marshal(result)
		fmt.Println(string(byt))
	}
}
