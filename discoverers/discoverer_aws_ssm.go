package discoverers

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/borderzero/discovery"
	"github.com/borderzero/discovery/utils"
	"golang.org/x/exp/slices"
)

const (
	defaultAwsSsmDiscovererDiscovererId = "aws_ssm_discoverer"
	defaultSsmGetAccountIdTimeout       = time.Second * 10
	defaultSsmPaginatorNextPageTimeout  = time.Second * 10
	defaultSsmPaginatorMaxResults       = 50 // api allows max of 50
)

// AwsSsmDiscoverer represents a discoverer for AWS SSM resources.
type AwsSsmDiscoverer struct {
	cfg aws.Config

	discovererId                string
	ssmPaginatorMaxResults      int
	ssmPaginatorNextPageTimeout time.Duration
	getAccountIdTimeout         time.Duration
}

// ensure AwsSsmDiscoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*AwsSsmDiscoverer)(nil)

// AwsSsmDiscovererOption represents a configuration option for an AwsSsmDiscoverer.
type AwsSsmDiscovererOption func(*AwsSsmDiscoverer)

// WithAwsSsmDiscovererDiscovererId is the AwsSsmDiscovererOption to set a non default discoverer id.
func WithAwsSsmDiscovererDiscovererId(discovererId string) AwsSsmDiscovererOption {
	return func(ssmd *AwsSsmDiscoverer) {
		ssmd.discovererId = discovererId
	}
}

// WithAwsSsmDiscovererGetAccountIdTimeout is the AwsSsmDiscovererOption
// to set a non default timeout for getting the aws account id.
func WithAwsSsmDiscovererGetAccountIdTimeout(timeout time.Duration) AwsSsmDiscovererOption {
	return func(ssmd *AwsSsmDiscoverer) {
		ssmd.getAccountIdTimeout = timeout
	}
}

// NewAwsSsmDiscoverer returns a new AwsSsmDiscoverer, initialized with the given options.
func NewAwsSsmDiscoverer(cfg aws.Config, opts ...AwsSsmDiscovererOption) *AwsSsmDiscoverer {
	ssmd := &AwsSsmDiscoverer{
		cfg: cfg,

		discovererId:                defaultAwsSsmDiscovererDiscovererId,
		getAccountIdTimeout:         defaultSsmGetAccountIdTimeout,
		ssmPaginatorNextPageTimeout: defaultSsmPaginatorNextPageTimeout,
		ssmPaginatorMaxResults:      defaultSsmPaginatorMaxResults,
	}
	for _, opt := range opts {
		opt(ssmd)
	}
	return ssmd
}

// Discover runs the AwsSsmDiscoverer.
func (ssmd *AwsSsmDiscoverer) Discover(ctx context.Context) *discovery.Result {
	result := discovery.NewResult(ssmd.discovererId)
	defer result.Done()

	awsAccountId, err := utils.AwsAccountIdFromConfig(ctx, ssmd.cfg, ssmd.getAccountIdTimeout)
	if err != nil {
		result.AddError(fmt.Errorf("failed to get AWS account ID from AWS configuration: %w", err))
		return result
	}

	ssmClient := ssm.NewFromConfig(ssmd.cfg)
	paginator := ssm.NewDescribeInstanceInformationPaginator(
		ssmClient,
		&ssm.DescribeInstanceInformationInput{
			MaxResults: aws.Int32(int32(ssmd.ssmPaginatorMaxResults)),
		},
	)

	for paginator.HasMorePages() {
		resources, err := processSsmDescribeInstanceInformationPage(
			ctx,
			paginator,
			ssmd.ssmPaginatorNextPageTimeout,
			time.Minute*10, // fixme: make input opt
			[]types.PingStatus{types.PingStatusOnline}, // fixme: make input opt
			ssmd.cfg.Region,
			awsAccountId,
		)
		if err != nil {
			result.AddError(fmt.Errorf("failed to get instance information from SSM: %w", err))
			return result
		}
		result.AddResources(resources...)
	}

	return result
}

func processSsmDescribeInstanceInformationPage(
	ctx context.Context,
	paginator *ssm.DescribeInstanceInformationPaginator,
	nextPageTimeout time.Duration,
	includeIfPingedSince time.Duration,
	includePingStatuses []types.PingStatus,
	awsRegion string,
	awsAccountId string,
) ([]discovery.Resource, error) {

	nextPageCtx, cancel := context.WithTimeout(ctx, nextPageTimeout)
	defer cancel()

	describeInstanceInformationOutput, err := paginator.NextPage(nextPageCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get page from AWS SSM describe instance information paginator: %w", err)
	}

	resources := []discovery.Resource{}
	for _, instanceInformation := range describeInstanceInformationOutput.InstanceInformationList {
		// ignore instances not seen in a given time
		if aws.ToTime(instanceInformation.LastPingDateTime).Before(time.Now().Add(-1 * includeIfPingedSince)) {
			continue
		}
		// ignore instances with an excluded ping status
		if !slices.Contains(includePingStatuses, instanceInformation.PingStatus) {
			continue
		}
		// build resource
		awsBaseDetails := discovery.AwsBaseDetails{
			AwsRegion:    awsRegion,
			AwsAccountId: awsAccountId,
			AwsArn: fmt.Sprintf(
				"arn:aws:ec2:%s:%s:instance/%s",
				awsRegion,
				awsAccountId,
				aws.ToString(instanceInformation.InstanceId),
			),
		}
		ssmTarget := &discovery.AwsSsmTargetDetails{
			AwsBaseDetails: awsBaseDetails,
			InstanceId:     aws.ToString(instanceInformation.InstanceId),
		}
		resources = append(resources, discovery.Resource{
			ResourceType:        discovery.ResourceTypeAwsSsmTarget,
			AwsSsmTargetDetails: ssmTarget,
		})
	}

	return resources, nil

}
