package discoverers

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/borderzero/border0-go/lib/types/maps"
	"github.com/borderzero/border0-go/lib/types/pointer"
	"github.com/borderzero/border0-go/lib/types/set"
	"github.com/borderzero/border0-go/lib/types/slice"
	"github.com/borderzero/discovery"
	"github.com/borderzero/discovery/utils"
)

const (
	defaultAwsEc2DiscovererDiscovererId             = "aws_ec2_discoverer"
	defaultAwsEc2SsmStatusCheckEnabled              = true
	defaultAwsEc2DiscovererGetAccountIdTimeout      = time.Second * 10
	defaultAwsEc2DiscovererDescribeInstancesTimeout = time.Second * 10
)

var (
	defaultAwsEc2DiscovererIncludedInstanceStates = set.New(
		types.InstanceStateNamePending,
		types.InstanceStateNameRunning,
		types.InstanceStateNameShuttingDown,
		types.InstanceStateNameTerminated,
		types.InstanceStateNameStopping,
		types.InstanceStateNameStopped,
	)
)

// AwsEc2Discoverer represents a discoverer for AWS EC2 resources.
type AwsEc2Discoverer struct {
	cfg aws.Config

	discovererId             string
	ssmStatusCheckEnabled    bool
	getAccountIdTimeout      time.Duration
	describeInstancesTimeout time.Duration
	includedInstanceStates   set.Set[types.InstanceStateName]
	inclusionInstanceTags    map[string][]string
	exclusionInstanceTags    map[string][]string
}

// ensure AwsEc2Discoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*AwsEc2Discoverer)(nil)

// AwsEc2DiscovererOption represents a configuration option for an AwsEc2Discoverer.
type AwsEc2DiscovererOption func(*AwsEc2Discoverer)

// WithAwsEc2DiscovererDiscovererId is the AwsEc2DiscovererOption to set a non default discoverer id.
func WithAwsEc2DiscovererDiscovererId(discovererId string) AwsEc2DiscovererOption {
	return func(ec2d *AwsEc2Discoverer) {
		ec2d.discovererId = discovererId
	}
}

// WithAwsEc2DiscovererSsmStatusCheck is the AwsEc2DiscovererOption
// to enable/disable checking instances' status with SSM.
func WithAwsEc2DiscovererSsmStatusCheck(enabled bool) AwsEc2DiscovererOption {
	return func(ec2d *AwsEc2Discoverer) { ec2d.ssmStatusCheckEnabled = enabled }
}

// WithAwsEc2DiscovererDiscovererId is the AwsEc2DiscovererOption
// to set a non default timeout for getting the aws account id.
func WithAwsEc2DiscovererGetAccountIdTimeout(timeout time.Duration) AwsEc2DiscovererOption {
	return func(ec2d *AwsEc2Discoverer) {
		ec2d.getAccountIdTimeout = timeout
	}
}

// WithAwsEc2DiscovererDescribeInstancesTimeout is the AwsEc2DiscovererOption
// to set a non default timeout for the describe instnaces api call.
func WithAwsEc2DiscovererDescribeInstancesTimeout(timeout time.Duration) AwsEc2DiscovererOption {
	return func(ec2d *AwsEc2Discoverer) {
		ec2d.describeInstancesTimeout = timeout
	}
}

// WithAwsEc2DiscovererIncludedInstanceStates is the AwsEc2DiscovererOption
// to set a non default list of states for instances to include in results.
func WithAwsEc2DiscovererIncludedInstanceStates(states ...types.InstanceStateName) AwsEc2DiscovererOption {
	return func(ec2d *AwsEc2Discoverer) {
		ec2d.includedInstanceStates = set.New(states...)
	}
}

// WithAwsEc2DiscovererInclusionInstanceTags is the AwsEc2DiscovererOption
// to set the inclusion tags filter for instances to include in results.
func WithAwsEc2DiscovererInclusionInstanceTags(tags map[string][]string) AwsEc2DiscovererOption {
	return func(ec2d *AwsEc2Discoverer) {
		ec2d.inclusionInstanceTags = tags
	}
}

// WithAwsEc2DiscovererExclusionInstanceTags is the AwsEc2DiscovererOption
// to set the exclusion tags filter for instances to exclude in results.
func WithAwsEc2DiscovererExclusionInstanceTags(tags map[string][]string) AwsEc2DiscovererOption {
	return func(ec2d *AwsEc2Discoverer) {
		ec2d.exclusionInstanceTags = tags
	}
}

// NewEngine returns a new AwsEc2Discoverer, initialized with the given options.
func NewAwsEc2Discoverer(cfg aws.Config, opts ...AwsEc2DiscovererOption) *AwsEc2Discoverer {
	ec2d := &AwsEc2Discoverer{
		cfg: cfg,

		discovererId:             defaultAwsEc2DiscovererDiscovererId,
		ssmStatusCheckEnabled:    defaultAwsEc2SsmStatusCheckEnabled,
		getAccountIdTimeout:      defaultAwsEc2DiscovererGetAccountIdTimeout,
		describeInstancesTimeout: defaultAwsEc2DiscovererDescribeInstancesTimeout,
		includedInstanceStates:   defaultAwsEc2DiscovererIncludedInstanceStates,
		inclusionInstanceTags:    nil,
		exclusionInstanceTags:    nil,
	}
	for _, opt := range opts {
		opt(ec2d)
	}
	return ec2d
}

// Discover runs the AwsEc2Discoverer.
func (ec2d *AwsEc2Discoverer) Discover(ctx context.Context) *discovery.Result {
	result := discovery.NewResult(ec2d.discovererId)
	defer result.Done()

	awsAccountId, err := utils.AwsAccountIdFromConfig(ctx, ec2d.cfg, ec2d.getAccountIdTimeout)
	if err != nil {
		result.AddError(fmt.Errorf("failed to get AWS account ID from AWS configuration: %w", err))
		return result
	}

	ssmInstanceStatuses := make(map[string]bool)
	if ec2d.ssmStatusCheckEnabled {
		if err := ec2d.collectSsmInstanceStatuses(ctx, ssmInstanceStatuses); err != nil {
			result.AddError(fmt.Errorf("failed to collect SSM instance statuses: %w", err))
			return result
		}
	}

	// describe ec2 instances
	describeInstancesCtx, cancel := context.WithTimeout(ctx, ec2d.describeInstancesTimeout)
	defer cancel()

	// TODO: use paginator
	ec2Client := ec2.NewFromConfig(ec2d.cfg)
	describeInstancesOutput, err := ec2Client.DescribeInstances(describeInstancesCtx, &ec2.DescribeInstancesInput{})
	if err != nil {
		result.AddError(fmt.Errorf("failed to describe ec2 instances: %w", err))
		return result
	}

	// filter and build resources
	for _, reservation := range describeInstancesOutput.Reservations {
		for _, instance := range reservation.Instances {
			// ignore instances with no state
			if instance.State == nil {
				continue // NOTE: this should emit a warning.
			}
			// ignore instances with un-included instance states
			if !ec2d.includedInstanceStates.Has(instance.State.Name) {
				continue
			}
			// ignore instances that don't satisfy tag conditions
			if !maps.MatchesFilters(
				slice.Map(
					instance.Tags,
					func(tag types.Tag) (string, string) {
						return aws.ToString(tag.Key), aws.ToString(tag.Value)
					},
				),
				ec2d.inclusionInstanceTags,
				ec2d.exclusionInstanceTags,
			) {
				continue
			}
			// build resource
			instanceId := aws.ToString(instance.InstanceId)
			awsBaseDetails := discovery.AwsBaseDetails{
				AwsRegion:    ec2d.cfg.Region,
				AwsAccountId: awsAccountId,
				AwsArn: fmt.Sprintf(
					"arn:aws:ec2:%s:%s:instance/%s",
					ec2d.cfg.Region,
					awsAccountId,
					instanceId,
				),
			}
			tags := map[string]string{}
			for _, t := range instance.Tags {
				tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
			}
			ssmInstanceStatus := discovery.Ec2InstanceSsmStatusNotChecked
			if ec2d.ssmStatusCheckEnabled {
				if online, associated := ssmInstanceStatuses[aws.ToString(instance.InstanceId)]; associated {
					if online {
						ssmInstanceStatus = discovery.Ec2InstanceSsmStatusOnline
					} else {
						ssmInstanceStatus = discovery.Ec2InstanceSsmStatusOffline
					}
				} else {
					ssmInstanceStatus = discovery.Ec2InstanceSsmStatusNotAssociated
				}
			}
			ec2InstanceDetails := &discovery.AwsEc2InstanceDetails{
				AwsBaseDetails:   awsBaseDetails,
				Tags:             tags,
				InstanceId:       aws.ToString(instance.InstanceId),
				ImageId:          aws.ToString(instance.ImageId),
				VpcId:            aws.ToString(instance.VpcId),
				SubnetId:         aws.ToString(instance.SubnetId),
				AvailabilityZone: aws.ToString(pointer.ValueOrZero(instance.Placement).AvailabilityZone),
				PrivateDnsName:   aws.ToString(instance.PrivateDnsName),
				PrivateIpAddress: aws.ToString(instance.PrivateIpAddress),
				PublicDnsName:    aws.ToString(instance.PublicDnsName),
				PublicIpAddress:  aws.ToString(instance.PublicIpAddress),
				InstanceType:     string(instance.InstanceType),
				InstanceState:    string(pointer.ValueOrZero(instance.State).Name),

				InstanceSsmStatus: ssmInstanceStatus,
			}
			result.AddResources(discovery.Resource{
				ResourceType:          discovery.ResourceTypeAwsEc2Instance,
				AwsEc2InstanceDetails: ec2InstanceDetails,
			})
		}
	}

	return result
}

func (ec2d *AwsEc2Discoverer) collectSsmInstanceStatuses(
	ctx context.Context,
	statuses map[string]bool,
) error {
	paginator := ssm.NewDescribeInstanceInformationPaginator(
		ssm.NewFromConfig(ec2d.cfg),
		&ssm.DescribeInstanceInformationInput{},
	)
	for paginator.HasMorePages() {
		err := processSsmInstanceInformationPage(
			ctx,
			statuses,
			paginator,
		)
		if err != nil {
			return fmt.Errorf("failed to process SSM instance information page: %v", err)
		}
	}
	return nil
}

func processSsmInstanceInformationPage(
	ctx context.Context,
	statuses map[string]bool,
	paginator *ssm.DescribeInstanceInformationPaginator,
) error {
	describeInstanceInformationOutput, err := paginator.NextPage(ctx)
	if err != nil {
		return fmt.Errorf("failed to get next page: %v", err)
	}
	for _, instanceInfo := range describeInstanceInformationOutput.InstanceInformationList {
		// note: presense in the response implies that the SSM api knows
		// about this instance (so it is associated with SSM). We add it
		// to the map with an offline status (false).
		statuses[aws.ToString(instanceInfo.InstanceId)] = false

		onlinePingStatus := instanceInfo.PingStatus == ssmtypes.PingStatusOnline
		timeSinceLastPing := time.Since(aws.ToTime(instanceInfo.LastPingDateTime))

		if onlinePingStatus && timeSinceLastPing <= 10*time.Minute {
			statuses[aws.ToString(instanceInfo.InstanceId)] = true // mark online
		}
	}
	return nil
}
