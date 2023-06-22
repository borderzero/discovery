package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/borderzero/discovery"
	"github.com/borderzero/discovery/discoverers"
	"github.com/borderzero/discovery/engines"
)

func main() {
	ctx := context.Background()

	engine := engines.NewOneOffEngine(
		engines.OneOffEngineOptionWithDiscoverers(
			discoverers.NewNaiveNetworkDiscoverer( /* ... opts ... */ ),
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
