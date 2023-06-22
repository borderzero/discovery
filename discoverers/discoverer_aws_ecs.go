package discoverers

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/borderzero/discovery"
	"github.com/borderzero/discovery/utils"
	"golang.org/x/exp/slices"
)

const (
	defaultAwsEcsDiscovererDiscovererId        = "aws_ecs_discoverer"
	defaultAwsEcsDiscovererGetAccountIdTimeout = time.Second * 10
)

// AwsEcsDiscoverer represents a discoverer for AWS ECS resources.
type AwsEcsDiscoverer struct {
	cfg aws.Config

	discovererId        string
	getAccountIdTimeout time.Duration
}

// ensure AwsEcsDiscoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*AwsEcsDiscoverer)(nil)

// AwsEcsDiscovererOption represents a configuration option for an AwsEcsDiscoverer.
type AwsEcsDiscovererOption func(*AwsEcsDiscoverer)

// WithAwsEcsDiscovererDiscovererId is the AwsEcsDiscovererOption to set a non default discoverer id.
func WithAwsEcsDiscovererDiscovererId(discovererId string) AwsEcsDiscovererOption {
	return func(ecsd *AwsEcsDiscoverer) {
		ecsd.discovererId = discovererId
	}
}

// WithAwsEcsDiscovererGetAccountIdTimeout is the AwsEcsDiscovererOption
// to set a non default timeout for getting the aws account id.
func WithAwsEcsDiscovererGetAccountIdTimeout(timeout time.Duration) AwsEcsDiscovererOption {
	return func(ecsd *AwsEcsDiscoverer) {
		ecsd.getAccountIdTimeout = timeout
	}
}

// NewAwsEcsDiscoverer returns a new AwsEcsDiscoverer.
func NewAwsEcsDiscoverer(cfg aws.Config, opts ...AwsEcsDiscovererOption) *AwsEcsDiscoverer {
	ecsd := &AwsEcsDiscoverer{
		cfg: cfg,

		discovererId:        defaultAwsEcsDiscovererDiscovererId,
		getAccountIdTimeout: defaultAwsEcsDiscovererGetAccountIdTimeout,
	}
	for _, opt := range opts {
		opt(ecsd)
	}
	return ecsd
}

// Discover runs the AwsEcsDiscoverer and closes the channels after a single run.
func (ecsd *AwsEcsDiscoverer) Discover(ctx context.Context) *discovery.Result {
	result := discovery.NewResult(ecsd.discovererId)
	defer result.Done()

	awsAccountId, err := utils.AwsAccountIdFromConfig(ctx, ecsd.cfg, ecsd.getAccountIdTimeout)
	if err != nil {
		result.AddError(fmt.Errorf("failed to get AWS account ID from AWS configuration: %w", err))
		return result
	}

	// describe ecs clusters
	ecsClient := ecs.NewFromConfig(ecsd.cfg)
	// TODO: new context with timeout for list clusters
	// TODO: use paginator
	listClustersOutput, err := ecsClient.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		result.AddError(fmt.Errorf("failed to list ecs clusters: %w", err))
		return result
	}
	// TODO: new context with timeout for describe clusters
	// TODO: use paginator
	describeClustersOutput, err := ecsClient.DescribeClusters(ctx, &ecs.DescribeClustersInput{Clusters: listClustersOutput.ClusterArns})
	if err != nil {
		result.AddError(fmt.Errorf("failed to describe ecs clusters: %w", err))
		return result
	}

	// filter and build resources
	for _, cluster := range describeClustersOutput.Clusters {
		// ignore unavailable clusters
		if !slices.Contains([]string{"ACTIVE", "PROVISIONING"}, aws.ToString(cluster.Status)) {
			continue
		}
		// build resource
		awsBaseDetails := discovery.AwsBaseDetails{
			AwsRegion:    ecsd.cfg.Region,
			AwsAccountId: awsAccountId,
			AwsArn:       aws.ToString(cluster.ClusterArn),
		}
		// Note: might need to make a few api calls for each cluster...
		// - DescribeServices
		// - DescribeTasks
		// - DescribeContainerInstances
		tags := map[string]string{}
		for _, t := range cluster.Tags {
			tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
		}
		ecsClusterDetails := &discovery.AwsEcsClusterDetails{
			AwsBaseDetails: awsBaseDetails,
			Tags:           tags,
			ClusterName:    aws.ToString(cluster.ClusterName),
			// TODO: add details
		}
		result.AddResources(discovery.Resource{
			ResourceType:         discovery.ResourceTypeAwsEcsCluster,
			AwsEcsClusterDetails: ecsClusterDetails,
		})
	}

	return result
}
