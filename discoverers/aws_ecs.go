package discoverers

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/borderzero/discovery"
	"golang.org/x/exp/slices"
)

// AwsEcsDiscoverer represents a discoverer for AWS ECS resources.
type AwsEcsDiscoverer struct {
	cfg aws.Config
}

// ensure AwsEcsDiscoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*AwsEcsDiscoverer)(nil)

// AwsEcsDiscovererOption is an input option for the AwsEcsDiscoverer constructor.
type AwsEcsDiscovererOption func(*AwsEcsDiscoverer)

// NewAwsEcsDiscoverer returns a new AwsEcsDiscoverer, initialized with the given options.
func NewAwsEcsDiscoverer(cfg aws.Config, opts ...AwsEcsDiscovererOption) *AwsEcsDiscoverer {
	ecsd := &AwsEcsDiscoverer{cfg: cfg}
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

	// get caller identity
	gciCtx, gciCtxCancel := context.WithTimeout(ctx, time.Second*2)
	defer gciCtxCancel()
	stsClient := sts.NewFromConfig(ecsd.cfg)
	getCallerIdentityOutput, err := stsClient.GetCallerIdentity(gciCtx, &sts.GetCallerIdentityInput{})
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to get caller identity via sts: %v", err))
		return
	}
	awsAccountId := aws.ToString(getCallerIdentityOutput.Account)

	// describe ecs clusters
	ecsClient := ecs.NewFromConfig(ecsd.cfg)
	// TODO: new context with timeout for list clusters
	listClustersOutput, err := ecsClient.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to list ecs clusters: %v", err))
		return
	}
	// TODO: new context with timeout for describe clusters
	describeClustersOutput, err := ecsClient.DescribeClusters(ctx, &ecs.DescribeClustersInput{Clusters: listClustersOutput.ClusterArns})
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to list ecs clusters: %v", err))
		return
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
		result.Resources = append(result.Resources, discovery.Resource{
			ResourceType:         discovery.ResourceTypeAwsEcsCluster,
			AwsEcsClusterDetails: ecsClusterDetails,
		})
	}
}
