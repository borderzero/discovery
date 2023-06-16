# discovery

[![Go Report Card](https://goreportcard.com/badge/github.com/borderzero/discovery)](https://goreportcard.com/report/github.com/borderzero/discovery)
[![Documentation](https://godoc.org/github.com/borderzero/discovery?status.svg)](https://godoc.org/github.com/borderzero/discovery)
[![license](https://img.shields.io/github/license/borderzero/discovery.svg)](https://github.com/borderzero/discovery/blob/master/LICENSE)

Border0 service discovery framework and library.

### Example: Discover EC2, ECS, and RDS Resources

Assume that the following variables are defined as follows:

```
ctx := context.Background()

cfg, err := config.LoadDefaultConfig(ctx)
if err != nil {
	// handle error
}
```

Then,

```
// initialize a new one off engine
engine := engines.NewOneOffEngine(
	engines.OneOffEngineOptionWithDiscoverers(
		discoverers.NewAwsEc2Discoverer(cfg),
		discoverers.NewAwsEcsDiscoverer(cfg),
		discoverers.NewAwsRdsDiscoverer(cfg),
		// ... LAN, docker, k8s, gcp compute, azure vms, etc ...
	),
)

// create channels for discovery results
results := make(chan *discovery.Result, 10)

// run engine
go engine.Run(ctx, results)

// process results as they come in
for result := range results {
	// ... do something ...
}
```

### Example: Continuously Discover EC2, ECS, and RDS Resources

Assume that the following variables are defined as follows:

> Assume that ctx (type `context.Context`) is defined by some upstream code

```
cfg, err := config.LoadDefaultConfig(ctx)
if err != nil {
	// handle error
}
```

Then,

```
// initialize a new continuous engine
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

// create channels for discovery results
results := make(chan *discovery.Result, 10)

// run engine
go engine.Run(ctx, results)

// process results as they come in
for result := range results {
	// ... do something ...
}
```

### Example: Discover EC2 Instances In Multiple AWS Regions

Assume that the following variables are defined as follows:

```
awsRegions := []string{"us-east-1", "us-east-2", "us-west-2", "eu-west-1"}

ctx := context.Background()
cfg, err := config.LoadDefaultConfig(ctx)
if err != nil {
	// handle error
}
```

Then,

```
// define an ec2 discoverer for each region
ds := []discovery.Discoverer{}
for _, region := range regions {
	cfg.Region = region

	ds = append(ds, discoverers.NewAwsEc2Discoverer(cfg))
}

// initialize a new one off engine with the discoverers
engine := engines.NewOneOffEngine(
	engines.OneOffEngineOptionWithDiscoverers(ds...),
)

// create channels for discovery results
results := make(chan *discovery.Result, 10)

// run engine
go engine.Run(ctx, results)

// process results as they come in
for result := range results {
	// ... do something ...
}
```

