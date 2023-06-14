package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/borderzero/discovery"
	"github.com/borderzero/discovery/discoverers"
	"github.com/borderzero/discovery/engines"
)

func main() {
	awsRegions := []string{
		"af-south-1",
		"ap-east-1",
		"ap-northeast-1",
		"ap-northeast-2",
		"ap-south-1",
		"ap-southeast-1",
		"ap-southeast-2",
		"ca-central-1",
		"eu-central-1",
		"eu-north-1",
		"eu-south-1",
		"eu-west-1",
		"eu-west-2",
		"eu-west-3",
		"me-south-1",
		"sa-east-1",
		"us-east-1",
		"us-east-2",
		"us-west-1",
		"us-west-2",
	}

	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("unable to load SDK config, %w", err)
	}

	ds := []discovery.Discoverer{}
	for _, region := range awsRegions {
		cfg.Region = region
		ds = append(ds, discoverers.NewAwsEc2Discoverer(cfg))
	}

	engine := engines.NewOneOffEngine(
		engines.OneOffEngineOptionWithDiscoverers(ds...),
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
