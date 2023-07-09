package discoverers

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/borderzero/border0-go/lib/types/set"
	"github.com/borderzero/border0-go/lib/types/slice"
	"github.com/borderzero/discovery"
	"github.com/borderzero/discovery/utils"
)

const (
	defaultAwsEcsDiscovererDiscovererId        = "aws_ecs_discoverer"
	defaultAwsEcsDiscovererGetAccountIdTimeout = time.Second * 10
)

var (
	defaultAwsEcsDiscovererIncludedClusterStatuses = set.New("ACTIVE", "PROVISIONING")
)

// AwsEcsDiscoverer represents a discoverer for AWS ECS resources.
type AwsEcsDiscoverer struct {
	cfg aws.Config

	discovererId            string
	getAccountIdTimeout     time.Duration
	includedClusterStatuses set.Set[string]
	inclusionClusterTags    map[string][]string
	exclusionClusterTags    map[string][]string
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

// WithAwsEcsDiscovererIncludedClusterStatuses is the AwsEcsDiscovererOption
// to set a non default list of statuses for clusters to include in results.
func WithAwsEcsDiscovererIncludedClusterStatuses(statuses ...string) AwsEcsDiscovererOption {
	return func(ecsd *AwsEcsDiscoverer) {
		ecsd.includedClusterStatuses = set.New(statuses...)
	}
}

// WithAwsEcsDiscovererInclusionClusterTags is the AwsEcsDiscovererOption
// to set the inclusion tags filter for clusters to include in results.
func WithAwsEcsDiscovererInclusionClusterTags(tags map[string][]string) AwsEcsDiscovererOption {
	return func(ecsd *AwsEcsDiscoverer) {
		ecsd.inclusionClusterTags = tags
	}
}

// WithAwsEcsDiscovererExclusionClusterTags is the AwsEcsDiscovererOption
// to set the inclusion tags filter for clusters to exclude in results.
func WithAwsEcsDiscovererExclusionClusterTags(tags map[string][]string) AwsEcsDiscovererOption {
	return func(ecsd *AwsEcsDiscoverer) {
		ecsd.exclusionClusterTags = tags
	}
}

// NewAwsEcsDiscoverer returns a new AwsEcsDiscoverer.
func NewAwsEcsDiscoverer(cfg aws.Config, opts ...AwsEcsDiscovererOption) *AwsEcsDiscoverer {
	ecsd := &AwsEcsDiscoverer{
		cfg: cfg,

		discovererId:            defaultAwsEcsDiscovererDiscovererId,
		getAccountIdTimeout:     defaultAwsEcsDiscovererGetAccountIdTimeout,
		includedClusterStatuses: defaultAwsEcsDiscovererIncludedClusterStatuses,
		inclusionClusterTags:    nil,
		exclusionClusterTags:    nil,
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
	describeClustersOutput, err := ecsClient.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: listClustersOutput.ClusterArns,
		Include:  []types.ClusterField{types.ClusterFieldTags},
	})
	if err != nil {
		result.AddError(fmt.Errorf("failed to describe ecs clusters: %w", err))
		return result
	}

	// filter and build resources
	for _, cluster := range describeClustersOutput.Clusters {
		// ignore clusters with no status
		if cluster.Status == nil {
			continue // NOTE: this should emit a warning.
		}
		// ignore clusters with un-included cluster states
		if !ecsd.includedClusterStatuses.Has(aws.ToString(cluster.Status)) {
			continue
		}
		// ignore clusters that don't satisfy tag conditions
		if !utils.KVMatchesFilters(
			slice.Map(
				cluster.Tags,
				func(tag types.Tag) (string, string) {
					return aws.ToString(tag.Key), aws.ToString(tag.Value)
				},
			),
			ecsd.inclusionClusterTags,
			ecsd.exclusionClusterTags,
		) {
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
			ClusterStatus:  aws.ToString(cluster.Status),
			// TODO: add details
		}
		result.AddResources(discovery.Resource{
			ResourceType:         discovery.ResourceTypeAwsEcsCluster,
			AwsEcsClusterDetails: ecsClusterDetails,
		})
	}

	return result
}
