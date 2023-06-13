package discoverers

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/borderzero/discovery"
)

// AwsEcsDiscoverer represents a discoverer for AWS ECS resources.
type AwsEcsDiscoverer struct {
	cfg          aws.Config
	awsAccountId string
}

// ensure AwsEcsDiscoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*AwsEcsDiscoverer)(nil)

// AwsEcsDiscovererOption is an input option for the AwsEcsDiscoverer constructor.
type AwsEcsDiscovererOption func(*AwsEcsDiscoverer)

// NewAwsEcsDiscoverer returns a new AwsEcsDiscoverer, initialized with the given options.
func NewAwsEcsDiscoverer(cfg aws.Config, awsAccountId string, opts ...AwsEcsDiscovererOption) *AwsEcsDiscoverer {
	ecsd := &AwsEcsDiscoverer{cfg: cfg, awsAccountId: awsAccountId}
	for _, opt := range opts {
		opt(ecsd)
	}
	return ecsd
}

// Discover runs the AwsEcsDiscoverer and closes the channels after a single run.
func (ecsd *AwsEcsDiscoverer) Discover(
	ctx context.Context,
	resources chan<- []discovery.Resource,
	errors chan<- error,
) {
	// discover routines are in charge of
	// closing their channels when done
	defer func() {
		close(resources)
		close(errors)
	}()

	ecsClient := ecs.NewFromConfig(ecsd.cfg)

	listClustersOutput, err := ecsClient.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		errors <- fmt.Errorf("failed to list ecs clusters: %w", err)
		return
	}

	discoveredResources := []discovery.Resource{}
	for _, clusterArn := range listClustersOutput.ClusterArns {
		// build resource
		awsBaseDetails := discovery.AwsBaseDetails{
			AwsRegion:    ecsd.cfg.Region,
			AwsAccountId: ecsd.awsAccountId,
			AwsArn:       clusterArn,
		}
		// Note: might need to make a few api calls for each cluster...
		// - DescribeCluster
		// - DescribeServices
		// - DescribeTasks
		// - DescribeContainerInstances
		ecsClusterDetails := &discovery.AwsEcsClusterDetails{
			AwsBaseDetails: awsBaseDetails,
			// TODO: add details
		}
		discoveredResources = append(discoveredResources, discovery.Resource{
			ResourceType:         discovery.ResourceTypeAwsEcsCluster,
			AwsEcsClusterDetails: ecsClusterDetails,
		})
	}

	resources <- discoveredResources
}
