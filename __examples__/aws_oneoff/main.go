package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/borderzero/discovery"
	"github.com/borderzero/discovery/discoverers"
	"github.com/borderzero/discovery/engines"
)

func main() {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("unable to load SDK config, %w", err)
	}

	engine := engines.NewOneOffEngine(
		engines.OneOffEngineOptionWithDiscoverers(
			discoverers.NewAwsEc2Discoverer(
				cfg,
				discoverers.WithAwsEc2DiscovererIncludedInstanceStates(
					types.InstanceStateNamePending,
					types.InstanceStateNameRunning,
				),
			),
			discoverers.NewAwsEcsDiscoverer(cfg),
			discoverers.NewAwsRdsDiscoverer(cfg),
			discoverers.NewAwsSsmDiscoverer(cfg),
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
