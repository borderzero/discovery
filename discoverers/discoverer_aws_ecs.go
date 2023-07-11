package discoverers

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/borderzero/border0-go/lib/types/maps"
	"github.com/borderzero/border0-go/lib/types/slice"
	"github.com/borderzero/discovery"
	"github.com/borderzero/discovery/utils"
)

const (
	defaultAwsEcsDiscovererDiscovererId        = "aws_ecs_discoverer"
	defaultAwsEcsDiscovererGetAccountIdTimeout = time.Second * 10
)

// AwsEcsDiscoverer represents a discoverer for AWS ECS resources.
type AwsEcsDiscoverer struct {
	cfg aws.Config

	discovererId         string
	getAccountIdTimeout  time.Duration
	inclusionServiceTags map[string][]string
	exclusionServiceTags map[string][]string
}

// ensure AwsEcsDiscoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*AwsEcsDiscoverer)(nil)

// AwsEcsDiscovererOption represents a configuration option for an AwsEcsDiscoverer.
type AwsEcsDiscovererOption func(*AwsEcsDiscoverer)

// WithAwsEcsDiscovererDiscovererId is the AwsEcsDiscovererOption to set a non default discoverer id.
func WithAwsEcsDiscovererDiscovererId(discovererId string) AwsEcsDiscovererOption {
	return func(ecsd *AwsEcsDiscoverer) { ecsd.discovererId = discovererId }
}

// WithAwsEcsDiscovererGetAccountIdTimeout is the AwsEcsDiscovererOption
// to set a non default timeout for getting the aws account id.
func WithAwsEcsDiscovererGetAccountIdTimeout(timeout time.Duration) AwsEcsDiscovererOption {
	return func(ecsd *AwsEcsDiscoverer) { ecsd.getAccountIdTimeout = timeout }
}

// WithAwsEcsDiscovererInclusionServiceTags is the AwsEcsDiscovererOption
// to set the inclusion tags filter for services to include in results.
func WithAwsEcsDiscovererInclusionServiceTags(tags map[string][]string) AwsEcsDiscovererOption {
	return func(ecsd *AwsEcsDiscoverer) { ecsd.inclusionServiceTags = tags }
}

// WithAwsEcsDiscovererExclusionServiceTags is the AwsEcsDiscovererOption
// to set the inclusion tags filter for services to exclude in results.
func WithAwsEcsDiscovererExclusionServiceTags(tags map[string][]string) AwsEcsDiscovererOption {
	return func(ecsd *AwsEcsDiscoverer) { ecsd.exclusionServiceTags = tags }
}

// NewAwsEcsDiscoverer returns a new AwsEcsDiscoverer.
func NewAwsEcsDiscoverer(cfg aws.Config, opts ...AwsEcsDiscovererOption) *AwsEcsDiscoverer {
	ecsd := &AwsEcsDiscoverer{
		cfg: cfg,

		discovererId:         defaultAwsEcsDiscovererDiscovererId,
		getAccountIdTimeout:  defaultAwsEcsDiscovererGetAccountIdTimeout,
		inclusionServiceTags: nil,
		exclusionServiceTags: nil,
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
		result.AddErrorf("failed to get AWS account ID from AWS configuration: %v", err)
		return result
	}

	ecsClient := ecs.NewFromConfig(ecsd.cfg)
	paginator := ecs.NewListClustersPaginator(ecsClient, &ecs.ListClustersInput{})
	for paginator.HasMorePages() {
		ok := ecsd.processEcsListClustersPage(
			ctx,
			ecsClient,
			paginator,
			result,
			awsAccountId,
		)
		if !ok {
			break
		}
	}

	return result
}

func (ecsd *AwsEcsDiscoverer) processEcsListClustersPage(
	ctx context.Context,
	ecsClient *ecs.Client,
	listClustersPaginator *ecs.ListClustersPaginator,
	result *discovery.Result,
	awsAccountId string,
) bool {
	listClustersOutput, err := listClustersPaginator.NextPage(ctx)
	if err != nil {
		result.AddErrorf("failed to list ecs clusters: %v", err)
		return false
	}
	if len(listClustersOutput.ClusterArns) == 0 {
		return true
	}

	for _, clusterArn := range listClustersOutput.ClusterArns {
		paginator := ecs.NewListServicesPaginator(
			ecsClient,
			&ecs.ListServicesInput{
				Cluster: aws.String(clusterArn),

				// ecs describe services allows only describing 10 at a
				// time so we get services in batches of (at most) 10
				MaxResults: aws.Int32(10),
			},
		)
		for paginator.HasMorePages() {
			ok := ecsd.processEcsListServicesPage(
				ctx,
				ecsClient,
				clusterArn,
				paginator,
				result,
				awsAccountId,
			)
			if !ok {
				return false
			}
		}
	}

	return true
}

func (ecsd *AwsEcsDiscoverer) processEcsListServicesPage(
	ctx context.Context,
	ecsClient *ecs.Client,
	clusterArn string,
	listServicesPaginator *ecs.ListServicesPaginator,
	result *discovery.Result,
	awsAccountId string,
) bool {
	listServicesOutput, err := listServicesPaginator.NextPage(ctx)
	if err != nil {
		result.AddErrorf("failed to list ecs services: %v", err)
		return false
	}
	if len(listServicesOutput.ServiceArns) == 0 {
		return true
	}

	describeServicesOutput, err := ecsClient.DescribeServices(
		ctx,
		&ecs.DescribeServicesInput{
			Cluster:  aws.String(clusterArn),
			Services: listServicesOutput.ServiceArns,
			Include:  []types.ServiceField{types.ServiceFieldTags},
		},
	)
	if err != nil {
		result.AddErrorf("failed to describe ecs services: %v", err)
		return false
	}

	for _, failure := range describeServicesOutput.Failures {
		result.AddWarning(fmt.Sprintf("received a failure when describing ecs services: %v", failure))
	}
	for _, service := range describeServicesOutput.Services {
		ecsd.processEcsService(
			ctx,
			&service,
			result,
			awsAccountId,
		)
	}

	return true
}

func (ecsd *AwsEcsDiscoverer) processEcsService(
	ctx context.Context,
	service *types.Service,
	result *discovery.Result,
	awsAccountId string,
) {
	// ignore services that don't satisfy tag conditions
	if !maps.MatchesFilters(
		slice.Map(
			service.Tags,
			func(tag types.Tag) (string, string) {
				return aws.ToString(tag.Key), aws.ToString(tag.Value)
			},
		),
		ecsd.inclusionServiceTags,
		ecsd.exclusionServiceTags,
	) {
		return
	}
	// build resource
	awsBaseDetails := discovery.AwsBaseDetails{
		AwsRegion:    ecsd.cfg.Region,
		AwsAccountId: awsAccountId,
		AwsArn:       aws.ToString(service.ServiceArn),
	}
	tags := map[string]string{}
	for _, t := range service.Tags {
		tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
	}
	ecsServiceDetails := &discovery.AwsEcsServiceDetails{
		AwsBaseDetails:       awsBaseDetails,
		ClusterArn:           aws.ToString(service.ClusterArn),
		ServiceName:          aws.ToString(service.ServiceName),
		TaskDefinition:       aws.ToString(service.TaskDefinition),
		EnableExecuteCommand: service.EnableExecuteCommand,
		Tags:                 tags,
	}
	result.AddResources(discovery.Resource{
		ResourceType:         discovery.ResourceTypeAwsEcsService,
		AwsEcsServiceDetails: ecsServiceDetails,
	})
}
