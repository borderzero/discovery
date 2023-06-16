package discoverers

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/borderzero/discovery"
	"github.com/borderzero/discovery/lib/types/slice"
)

const (
	defaultEcsListClustersTimeout     = time.Second * 5
	defaultEcsDescribeClustersTimeout = time.Second * 5
)

var defaultIncludedEcsClusterStatuses = []string{"ACTIVE", "PROVISIONING"}

// AwsEcsClient represents an entity capable of acting as the aws ecs API client.
type AwsEcsClient interface {
	ListClusters(context.Context, *ecs.ListClustersInput) (*ecs.ListClustersOutput, error)
	DescribeClusters(context.Context, *ecs.DescribeClustersInput) (*ecs.DescribeClustersOutput, error)
}

// AwsEcsDiscoverer represents a discoverer for AWS ECS resources.
type AwsEcsDiscoverer struct {
	awsEcsClient AwsEcsClient

	listClustersTimeout     time.Duration
	describeClustersTimeout time.Duration

	includeClusterStatuses []string
}

// ensure AwsEcsDiscoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*AwsEcsDiscoverer)(nil)

// NewAwsEcsDiscoverer returns a new AwsEcsDiscoverer, initialized with the given options.
func NewAwsEcsDiscoverer(awsEcsClient AwsEcsClient) *AwsEcsDiscoverer {
	return &AwsEcsDiscoverer{
		awsEcsClient:            awsEcsClient,
		listClustersTimeout:     defaultEcsListClustersTimeout,
		describeClustersTimeout: defaultEcsDescribeClustersTimeout,
		includeClusterStatuses:  defaultIncludedEcsClusterStatuses,
	}
}

// Discover runs the AwsEcsDiscoverer and closes the channels after a single run.
func (ecsd *AwsEcsDiscoverer) Discover(ctx context.Context) *discovery.Result {
	result := discovery.NewResult()
	defer result.Done()

	// list ecs clusters
	listClustersCtx, listClustersCtxCancel := context.WithTimeout(ctx, ecsd.listClustersTimeout)
	defer listClustersCtxCancel()
	listClustersOutput, err := ecsd.awsEcsClient.ListClusters(listClustersCtx, &ecs.ListClustersInput{})
	if err != nil {
		result.AddError(fmt.Errorf("failed to list ecs clusters: %w", err))
		return result
	}

	// describe ecs clusters
	describeClustersCtx, describeClustersCtxCancel := context.WithTimeout(ctx, ecsd.describeClustersTimeout)
	defer describeClustersCtxCancel()
	describeClustersOutput, err := ecsd.awsEcsClient.DescribeClusters(
		describeClustersCtx,
		&ecs.DescribeClustersInput{Clusters: listClustersOutput.ClusterArns},
	)
	if err != nil {
		result.AddError(fmt.Errorf("failed to describe ecs clusters: %w", err))
		return result
	}

	// filter and build resources
	for _, cluster := range describeClustersOutput.Clusters {
		// ignore unavailable clusters
		if !slice.Contains(ecsd.includeClusterStatuses, aws.ToString(cluster.Status)) {
			continue
		}
		// parse arn
		clusterArnString := aws.ToString(cluster.ClusterArn)
		clusterArn, err := arn.Parse(clusterArnString)
		if err != nil {
			result.AddError(fmt.Errorf("got an invalid ecs cluster arn \"%s\"", clusterArnString))
			continue
		}
		// build resource
		awsBaseDetails := discovery.AwsBaseDetails{
			AwsRegion:    clusterArn.Region,
			AwsAccountId: clusterArn.AccountID,
			AwsArn:       clusterArnString,
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
