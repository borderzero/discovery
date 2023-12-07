package discoverers

import (
	"context"
	"fmt"
	"sync"
	"time"

	cache "github.com/Code-Hex/go-generics-cache"
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
	defaultAwsEc2SsmStatusCheckRequired             = false
	defaultAwsEc2DiscovererGetAccountIdTimeout      = time.Second * 10
	defaultAwsEc2DiscovererDescribeInstancesTimeout = time.Second * 10
	defaultAwsEc2ReachabilityCheckEnabled           = true
	defaultAwsEc2ReachabilityCheckCacheCleanPeriod  = time.Minute * 30
	defaultAwsEc2ReachabilityCheckCacheTtl          = time.Second * 5 // barely any caching
	defaultAwsEc2ReachabilityRequired               = false
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
	defaultAwsEc2NetworkReachabilityCheckPorts = set.New(
		"22",   // default ssh port
		"80",   // default http port
		"443",  // default https port
		"3306", // default mysql port
		"5432", // default postgresql port
		"8080", // common http port
		"8443", // common https port
	)
)

// AwsEc2Discoverer represents a discoverer for AWS EC2 resources.
type AwsEc2Discoverer struct {
	cfg aws.Config

	discovererId             string
	ssmStatusCheckEnabled    bool
	ssmStatusCheckRequired   bool
	getAccountIdTimeout      time.Duration
	describeInstancesTimeout time.Duration
	includedInstanceStates   set.Set[types.InstanceStateName]
	inclusionInstanceTags    map[string][]string
	exclusionInstanceTags    map[string][]string

	networkReachabilityCheckEnabled       bool
	networkReachabilityCheckPorts         set.Set[string]
	networkReachabilityCheckCache         *cache.Cache[string, bool]
	networkReachabilityCheckCacheItemOpts []cache.ItemOption
	reachabilityRequired                  bool
}

// ensure AwsEc2Discoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*AwsEc2Discoverer)(nil)

// AwsEc2DiscovererOption represents a configuration option for an AwsEc2Discoverer.
type AwsEc2DiscovererOption func(*AwsEc2Discoverer)

// WithAwsEc2DiscovererDiscovererId is the AwsEc2DiscovererOption to set a non default discoverer id.
func WithAwsEc2DiscovererDiscovererId(discovererId string) AwsEc2DiscovererOption {
	return func(ec2d *AwsEc2Discoverer) { ec2d.discovererId = discovererId }
}

// WithAwsEc2DiscovererSsmStatusCheck is the AwsEc2DiscovererOption
// to enable/disable checking instances' status with SSM.
// If required is true, enabled is automatically set to true.
func WithAwsEc2DiscovererSsmStatusCheck(enabled, required bool) AwsEc2DiscovererOption {
	return func(ec2d *AwsEc2Discoverer) {
		ec2d.ssmStatusCheckEnabled = enabled || required
		ec2d.ssmStatusCheckRequired = required
	}
}

// WithAwsEc2DiscovererReachabilityCheck is the AwsEc2DiscovererOption
// to enable/disable checking instances' reachability via the network.
func WithAwsEc2DiscovererNetworkReachabilityCheck(enabled bool) AwsEc2DiscovererOption {
	return func(ec2d *AwsEc2Discoverer) { ec2d.networkReachabilityCheckEnabled = enabled }
}

// WithAwsEc2DiscovererReachabilityRequired is the AwsEc2DiscovererOption
// to exclude instances that are not reachable through any means from results.
// If required is true, enabled is automatically set to true.
func WithAwsEc2DiscovererReachabilityRequired(required bool) AwsEc2DiscovererOption {
	return func(ec2d *AwsEc2Discoverer) {
		ec2d.networkReachabilityCheckEnabled = ec2d.networkReachabilityCheckEnabled || required
		ec2d.reachabilityRequired = required
	}
}

// WithAwsEc2DiscovererNetworkReachabilityCheckCache is the AwsEc2DiscovererOption
// to set the network reachability check cache and new item options.
func WithAwsEc2DiscovererNetworkReachabilityCheckCache(cache *cache.Cache[string, bool], itemOpts ...cache.ItemOption) AwsEc2DiscovererOption {
	return func(ec2d *AwsEc2Discoverer) {
		ec2d.networkReachabilityCheckCache = cache
		ec2d.networkReachabilityCheckCacheItemOpts = itemOpts
	}
}

// WithAwsEc2DiscovererDiscovererId is the AwsEc2DiscovererOption
// to set a non default timeout for getting the aws account id.
func WithAwsEc2DiscovererGetAccountIdTimeout(timeout time.Duration) AwsEc2DiscovererOption {
	return func(ec2d *AwsEc2Discoverer) { ec2d.getAccountIdTimeout = timeout }
}

// WithAwsEc2DiscovererDescribeInstancesTimeout is the AwsEc2DiscovererOption
// to set a non default timeout for the describe instnaces api call.
func WithAwsEc2DiscovererDescribeInstancesTimeout(timeout time.Duration) AwsEc2DiscovererOption {
	return func(ec2d *AwsEc2Discoverer) { ec2d.describeInstancesTimeout = timeout }
}

// WithAwsEc2DiscovererIncludedInstanceStates is the AwsEc2DiscovererOption
// to set a non default list of states for instances to include in results.
func WithAwsEc2DiscovererIncludedInstanceStates(states ...types.InstanceStateName) AwsEc2DiscovererOption {
	return func(ec2d *AwsEc2Discoverer) { ec2d.includedInstanceStates = set.New(states...) }
}

// WithAwsEc2DiscovererInclusionInstanceTags is the AwsEc2DiscovererOption
// to set the inclusion tags filter for instances to include in results.
func WithAwsEc2DiscovererInclusionInstanceTags(tags map[string][]string) AwsEc2DiscovererOption {
	return func(ec2d *AwsEc2Discoverer) { ec2d.inclusionInstanceTags = tags }
}

// WithAwsEc2DiscovererExclusionInstanceTags is the AwsEc2DiscovererOption
// to set the exclusion tags filter for instances to exclude in results.
func WithAwsEc2DiscovererExclusionInstanceTags(tags map[string][]string) AwsEc2DiscovererOption {
	return func(ec2d *AwsEc2Discoverer) { ec2d.exclusionInstanceTags = tags }
}

// NewEngine returns a new AwsEc2Discoverer, initialized with the given options.
func NewAwsEc2Discoverer(cfg aws.Config, opts ...AwsEc2DiscovererOption) *AwsEc2Discoverer {
	ec2d := &AwsEc2Discoverer{
		cfg: cfg,

		discovererId:             defaultAwsEc2DiscovererDiscovererId,
		ssmStatusCheckEnabled:    defaultAwsEc2SsmStatusCheckEnabled,
		ssmStatusCheckRequired:   defaultAwsEc2SsmStatusCheckRequired,
		getAccountIdTimeout:      defaultAwsEc2DiscovererGetAccountIdTimeout,
		describeInstancesTimeout: defaultAwsEc2DiscovererDescribeInstancesTimeout,
		includedInstanceStates:   defaultAwsEc2DiscovererIncludedInstanceStates,
		inclusionInstanceTags:    nil,
		exclusionInstanceTags:    nil,

		networkReachabilityCheckEnabled: defaultAwsEc2ReachabilityCheckEnabled,
		networkReachabilityCheckPorts:   defaultAwsEc2NetworkReachabilityCheckPorts,
		networkReachabilityCheckCache: cache.New[string, bool](
			cache.WithJanitorInterval[string, bool](defaultAwsEc2ReachabilityCheckCacheCleanPeriod),
		),
		networkReachabilityCheckCacheItemOpts: []cache.ItemOption{
			cache.WithExpiration(defaultAwsEc2ReachabilityCheckCacheTtl),
		},
		reachabilityRequired: defaultAwsEc2ReachabilityRequired,
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
		result.AddErrorf("failed to get AWS account ID from AWS configuration: %v", err)
		return result
	}

	ssmInstanceStatuses := make(map[string]bool)
	ssmInstanceCheckSucceeded := false
	if ec2d.ssmStatusCheckEnabled {
		if err := ec2d.collectSsmInstanceStatuses(ctx, ssmInstanceStatuses); err != nil {
			if ec2d.ssmStatusCheckRequired {
				result.AddErrorf("failed to collect SSM instance statuses: %v", err)
				return result
			} else {
				result.AddWarningf("failed to collect SSM instance statuses: %v", err)
			}
		} else {
			ssmInstanceCheckSucceeded = true
		}
	}

	// describe ec2 instances
	describeInstancesCtx, cancel := context.WithTimeout(ctx, ec2d.describeInstancesTimeout)
	defer cancel()

	// TODO: use paginator
	ec2Client := ec2.NewFromConfig(ec2d.cfg)
	describeInstancesOutput, err := ec2Client.DescribeInstances(describeInstancesCtx, &ec2.DescribeInstancesInput{})
	if err != nil {
		result.AddErrorf("failed to describe ec2 instances: %v", err)
		return result
	}

	// wait group for reachability checks
	var wg sync.WaitGroup
	defer wg.Wait()

	// filter and build resources
	for _, reservation := range describeInstancesOutput.Reservations {
		for _, instance := range reservation.Instances {
			// ignore instances with no state
			if instance.State == nil {
				result.AddWarningf(
					"received an instance (id %s) with a nil value for state",
					aws.ToString(instance.InstanceId),
				)
				continue
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
			if ec2d.ssmStatusCheckEnabled && ssmInstanceCheckSucceeded {
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

			wg.Add(1)
			go ec2d.reachabilityCheckAndAdd(ctx, &wg, result, ec2InstanceDetails)
		}
	}

	return result
}

func (ec2d *AwsEc2Discoverer) reachabilityCheckAndAdd(
	ctx context.Context,
	wg *sync.WaitGroup,
	result *discovery.Result,
	ec2Details *discovery.AwsEc2InstanceDetails,
) {
	defer wg.Done()

	if ec2d.networkReachabilityCheckEnabled {
		var testswg sync.WaitGroup

		if ec2Details.PrivateDnsName != "" {
			testswg.Add(1)
			go func() {
				defer testswg.Done()
				ec2Details.PrivateDnsNameReachable = pointer.To(
					ec2d.reachabilityCheck(ctx, ec2Details.PrivateDnsName),
				)
			}()
		}

		if ec2Details.PrivateIpAddress != "" {
			testswg.Add(1)
			go func() {
				defer testswg.Done()
				ec2Details.PrivateIpAddressReachable = pointer.To(
					ec2d.reachabilityCheck(ctx, ec2Details.PrivateIpAddress),
				)
			}()
		}

		if ec2Details.PublicDnsName != "" {
			testswg.Add(1)
			go func() {
				defer testswg.Done()
				ec2Details.PublicDnsNameReachable = pointer.To(
					ec2d.reachabilityCheck(ctx, ec2Details.PublicDnsName),
				)
			}()
		}

		if ec2Details.PublicIpAddress != "" {
			testswg.Add(1)
			go func() {
				defer testswg.Done()
				ec2Details.PublicIpAddressReachable = pointer.To(
					ec2d.reachabilityCheck(ctx, ec2Details.PublicIpAddress),
				)
			}()
		}

		testswg.Wait()
	}

	if !ec2d.shouldIncludeInstance(ec2Details) {
		return
	}

	result.AddResources(discovery.Resource{
		ResourceType:          discovery.ResourceTypeAwsEc2Instance,
		AwsEc2InstanceDetails: ec2Details,
	})
}

func (ec2d *AwsEc2Discoverer) shouldIncludeInstance(
	ec2Details *discovery.AwsEc2InstanceDetails,
) bool {
	// include if reachability is not required to include instances
	if !ec2d.reachabilityRequired {
		return true
	}

	// include if reachable via SSM
	if ec2Details.InstanceSsmStatus == discovery.Ec2InstanceSsmStatusOnline {
		return true
	}

	// include if reachable via any private or public dnsname or ip
	if pointer.ValueOrZero(ec2Details.PublicDnsNameReachable) ||
		pointer.ValueOrZero(ec2Details.PrivateDnsNameReachable) ||
		pointer.ValueOrZero(ec2Details.PublicIpAddressReachable) ||
		pointer.ValueOrZero(ec2Details.PrivateIpAddressReachable) {
		return true
	}

	return false
}

func (ec2d *AwsEc2Discoverer) reachabilityCheck(
	ctx context.Context,
	target string,
) bool {
	cached, ok := ec2d.networkReachabilityCheckCache.Get(target)
	if ok {
		return cached
	}

	ips, err := targetToIps(target)
	if err != nil {
		return false
	}

	for _, ip := range ips {
		for _, port := range ec2d.networkReachabilityCheckPorts.Slice() {
			reachable := addressReachable(ctx, fmt.Sprintf("%s:%s", ip, port))
			if reachable {
				ec2d.networkReachabilityCheckCache.Set(
					target,
					true,
					ec2d.networkReachabilityCheckCacheItemOpts...,
				)
				return true
			}
		}
	}

	ec2d.networkReachabilityCheckCache.Set(
		target,
		false,
		ec2d.networkReachabilityCheckCacheItemOpts...,
	)
	return false
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
