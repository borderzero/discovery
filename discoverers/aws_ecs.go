package discoverers

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/borderzero/discovery"
)

// AwsEcsDiscoverer represents a discoverer for AWS ECS resources.
type AwsEcsDiscoverer struct {
	cfg          aws.Config
	awsAccountId string // FIXME: call aws sts get-caller-identity for this
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
	results chan<- *discovery.Result,
) {
	result := discovery.NewResult()
	defer func() {
		result.Metrics.EndedAt = time.Now()
		results <- result
		close(results)
	}()

	ecsClient := ecs.NewFromConfig(ecsd.cfg)

	listClustersOutput, err := ecsClient.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("failed to list ecs clusters: %w", err))
		return
	}

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
		result.Resources = append(result.Resources, discovery.Resource{
			ResourceType:         discovery.ResourceTypeAwsEcsCluster,
			AwsEcsClusterDetails: ecsClusterDetails,
		})
	}
}
