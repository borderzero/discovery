# discovery

[![Go Report Card](https://goreportcard.com/badge/github.com/borderzero/discovery)](https://goreportcard.com/report/github.com/borderzero/discovery)
[![Documentation](https://godoc.org/github.com/borderzero/discovery?status.svg)](https://godoc.org/github.com/borderzero/discovery)
[![license](https://img.shields.io/github/license/borderzero/discovery.svg)](https://github.com/borderzero/discovery/blob/master/LICENSE)

Border0 service discovery framework and library.

### Example: Discover EC2, ECS, and RDS Resources (In Parallel)

Assume that the following variables are defined as follows:

```
ctx := context.Background()
awsConfig := config.LoadDefaultConfig(ctx)
awsAccountId := "123456789012"
```

Then,

```
// initialize a new multiple upstream discoverer
discoverer := discoverers.NewMultipleUpstreamDiscoverer(
	discoverers.WithUpstreamDiscoverers(
		discoverers.NewAwsEc2Discoverer(awsConfig, awsAccountId),
		discoverers.NewAwsEcsDiscoverer(awsConfig, awsAccountId),
		discoverers.NewAwsRdsDiscoverer(awsConfig, awsAccountId),
		// ... docker, k8s, LAN, gcp compute, azure vms, etc ...
	),
)

// create channels for discovered resources and errors
resources := make(chan []discovery.Resource)
errors := make(chan error)

go func() {
	for _, batch := range resources {
		// do something with resources batch
	}
}()
go func() {
	for _, err := range errors {
		// do something with error
	}
}()

// run discoverer
discoverer.Discover(ctx, resources, errors)
```

### Example: Discover EC2 Instances In Multiple AWS Regions (In Parallel)

Assume that the following variables are defined as follows:

```
ctx := context.Background()
awsConfig := config.LoadDefaultConfig(ctx)
awsAccountId := "123456789012"
awsRegions := []string{"us-east-1", "us-east-2", "us-west-2", "eu-west-1"}
```

Then,

```
// initialize a discoverer of the "AwsEc2" implementation per AWS region
ds := []discovery.Discoverer{}
for _, region := range awsRegions {
	awsConfig.Region = region
	ds = append(ds, discoverers.NewAwsEc2Discoverer(awsConfig, awsAccountId))
}

// initialize a new discoverer of the "MultipleUpstreamDiscoverer" implementation
// (which runs multiple other discoverers of any kind in parallel)
d := discoverers.NewMultipleUpstreamDiscoverer(discoverers.WithUpstreamDiscoverers(ds...))

// create channels for discovered resources and errors
resources := make(chan []discovery.Resource)
errors := make(chan error)

go func() {
	for _, batch := range resources {
		// do something with resources batch
	}
}()
go func() {
	for _, err := range errors {
		// do something with error
	}
}()

// run discoverer
d.Discover(ctx, resources, errors)
```

