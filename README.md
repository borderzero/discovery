# discovery

[![Go Report Card](https://goreportcard.com/badge/github.com/borderzero/discovery)](https://goreportcard.com/report/github.com/borderzero/discovery)
[![Documentation](https://godoc.org/github.com/borderzero/discovery?status.svg)](https://godoc.org/github.com/borderzero/discovery)
[![license](https://img.shields.io/github/license/borderzero/discovery.svg)](https://github.com/borderzero/discovery/blob/master/LICENSE)

Border0 service discovery framework and library.

### Example: Discover EC2, ECS, and RDS Resources

Assume that the following variables are defined as follows:

```
ctx := context.Background()
cfg := config.LoadDefaultConfig(ctx)
```

Then,

```
// initialize a new multiple upstream discoverer
discoverer := discoverers.NewMultipleUpstreamDiscoverer(
	discoverers.WithUpstreamDiscoverers(
		discoverers.NewAwsEc2Discoverer(cfg),
		discoverers.NewAwsEcsDiscoverer(cfg),
		discoverers.NewAwsRdsDiscoverer(cfg),
		// ... docker, k8s, LAN, gcp compute, azure vms, etc ...
	),
)

// create channels for discovery results
results := make(chan *discovery.Result, 10)

// run discoverer
go d.Discover(ctx, results)

// process results as they come in
for result := range results {
	// ... do something ...
}
```

### Example: Discover EC2 Instances In Multiple AWS Regions (In Parallel)

Assume that the following variables are defined as follows:

```
ctx := context.Background()
cfg := config.LoadDefaultConfig(ctx)
awsRegions := []string{"us-east-1", "us-east-2", "us-west-2", "eu-west-1"}
```

Then,

```
// initialize a aws ec2 discoverer per AWS region
ds := []discovery.Discoverer{}
for _, region := range awsRegions {
	cfg.Region = region
	ds = append(ds, discoverers.NewAwsEc2Discoverer(cfg))
}

// initialize a new multiple upstream discoverer
discoverer := discoverers.NewMultipleUpstreamDiscoverer(
	discoverers.WithUpstreamDiscoverers(ds...),
)

// create channels for discovery results
results := make(chan *discovery.Result, 10)

// run discoverer
go d.Discover(ctx, results)

// process results as they come in
for result := range results {
	// ... do something ...
}
```

