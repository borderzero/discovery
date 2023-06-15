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

// NewAwsEcsDiscoverer returns a new AwsEcsDiscoverer, initialized with the given options.
func NewAwsEcsDiscoverer(cfg aws.Config) *AwsEcsDiscoverer {
	return &AwsEcsDiscoverer{cfg: cfg}
}

// Discover runs the AwsEcsDiscoverer and closes the channels after a single run.
func (ecsd *AwsEcsDiscoverer) Discover(ctx context.Context) *discovery.Result {
	result := discovery.NewResult()
	defer result.Done()

	// get caller identity
	gciCtx, gciCtxCancel := context.WithTimeout(ctx, time.Second*2)
	defer gciCtxCancel()
	stsClient := sts.NewFromConfig(ecsd.cfg)
	getCallerIdentityOutput, err := stsClient.GetCallerIdentity(gciCtx, &sts.GetCallerIdentityInput{})
	if err != nil {
		result.AddError(fmt.Errorf("failed to get caller identity via sts: %w", err))
		return result
	}
	awsAccountId := aws.ToString(getCallerIdentityOutput.Account)

	// describe ecs clusters
	ecsClient := ecs.NewFromConfig(ecsd.cfg)
	// TODO: new context with timeout for list clusters
	listClustersOutput, err := ecsClient.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		result.AddError(fmt.Errorf("failed to list ecs clusters: %w", err))
		return result
	}
	// TODO: new context with timeout for describe clusters
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
		result.AddResource(discovery.Resource{
			ResourceType:         discovery.ResourceTypeAwsEcsCluster,
			AwsEcsClusterDetails: ecsClusterDetails,
		})
	}

	return result
}
