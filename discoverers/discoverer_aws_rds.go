package discoverers

import (
	"context"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/borderzero/border0-go/lib/types/maps"
	"github.com/borderzero/border0-go/lib/types/set"
	"github.com/borderzero/border0-go/lib/types/slice"
	"github.com/borderzero/discovery"
	"github.com/borderzero/discovery/utils"
)

const (
	defaultAwsRdsDiscovererDiscovererId        = "aws_rds_discoverer"
	defaultAwsRdsDiscovererGetAccountIdTimeout = time.Second * 10
)

var (
	defaultAwsRdsDiscovererIncludedInstanceStatuses = set.New("creating", "backing-up", "starting", "available", "maintenance", "modifying")
)

// AwsRdsDiscoverer represents a discoverer for AWS RDS resources.
type AwsRdsDiscoverer struct {
	cfg aws.Config

	discovererId             string
	getAccountIdTimeout      time.Duration
	includedInstanceStatuses set.Set[string]
	inclusionInstanceTags    map[string][]string
	exclusionInstanceTags    map[string][]string
}

// ensure AwsRdsDiscoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*AwsRdsDiscoverer)(nil)

// AwsRdsDiscovererOption represents a configuration option for an AwsRdsDiscoverer.
type AwsRdsDiscovererOption func(*AwsRdsDiscoverer)

// WithAwsEcsDiscovererDiscovererId is the AwsRdsDiscovererOption to set a non default discoverer id.
func WithAwsRdsDiscovererDiscovererId(discovererId string) AwsRdsDiscovererOption {
	return func(rdsd *AwsRdsDiscoverer) { rdsd.discovererId = discovererId }
}

// WithAwsRdsDiscovererGetAccountIdTimeout is the AwsRdsDiscovererOption
// to set a non default timeout for getting the aws account id.
func WithAwsRdsDiscovererGetAccountIdTimeout(timeout time.Duration) AwsRdsDiscovererOption {
	return func(rdsd *AwsRdsDiscoverer) { rdsd.getAccountIdTimeout = timeout }
}

// WithAwsRdsDiscovererIncludedInstanceStatuses is the AwsRdsDiscovererOption
// to set a non default list of statuses for instances to include in results.
func WithAwsRdsDiscovererIncludedInstanceStatuses(statuses ...string) AwsRdsDiscovererOption {
	lowercased := slice.Transform(statuses, func(s string) string { return strings.ToLower(s) })
	return func(rdsd *AwsRdsDiscoverer) { rdsd.includedInstanceStatuses = set.New(lowercased...) }
}

// WithAwsRdsDiscovererInclusionInstanceTags is the AwsRdsDiscovererOption
// to set the inclusion tags filter for instances to include in results.
func WithAwsRdsDiscovererInclusionInstanceTags(tags map[string][]string) AwsRdsDiscovererOption {
	return func(rdsd *AwsRdsDiscoverer) { rdsd.inclusionInstanceTags = tags }
}

// WithAwsRdsDiscovererExclusionInstanceTags is the AwsRdsDiscovererOption
// to set the exclusion tags filter for instances to exclude in results.
func WithAwsRdsDiscovererExclusionInstanceTags(tags map[string][]string) AwsRdsDiscovererOption {
	return func(rdsd *AwsRdsDiscoverer) { rdsd.exclusionInstanceTags = tags }
}

// NewAwsRdsDiscoverer returns a new AwsRdsDiscoverer, initialized with the given options.
func NewAwsRdsDiscoverer(cfg aws.Config, opts ...AwsRdsDiscovererOption) *AwsRdsDiscoverer {
	rdsd := &AwsRdsDiscoverer{
		cfg: cfg,

		discovererId:             defaultAwsRdsDiscovererDiscovererId,
		getAccountIdTimeout:      defaultAwsRdsDiscovererGetAccountIdTimeout,
		includedInstanceStatuses: defaultAwsRdsDiscovererIncludedInstanceStatuses,
		inclusionInstanceTags:    nil,
		exclusionInstanceTags:    nil,
	}
	for _, opt := range opts {
		opt(rdsd)
	}
	return rdsd
}

// Discover runs the AwsRdsDiscoverer.
func (rdsd *AwsRdsDiscoverer) Discover(ctx context.Context) *discovery.Result {
	result := discovery.NewResult(rdsd.discovererId)
	defer result.Done()

	awsAccountId, err := utils.AwsAccountIdFromConfig(ctx, rdsd.cfg, rdsd.getAccountIdTimeout)
	if err != nil {
		result.AddErrorf("failed to get AWS account ID from AWS configuration: %v", err)
		return result
	}

	// describe rds instances
	rdsClient := rds.NewFromConfig(rdsd.cfg)
	// TODO: new context with timeout for describe instances
	describeDBInstancesOutput, err := rdsClient.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{})
	if err != nil {
		result.AddErrorf("failed to describe rds instances: %v", err)
		return result
	}

	// filter and build resources
	for _, instance := range describeDBInstancesOutput.DBInstances {
		// ignore instances with no status
		if instance.DBInstanceStatus == nil {
			continue // NOTE: this should emit a warning.
		}
		// ignore instances with un-included instance status
		if !rdsd.includedInstanceStatuses.Has(aws.ToString(instance.DBInstanceStatus)) {
			continue
		}
		// ignore rds db instances that don't satisfy tag conditions
		if !maps.MatchesFilters(
			slice.Map(
				instance.TagList,
				func(tag types.Tag) (string, string) {
					return aws.ToString(tag.Key), aws.ToString(tag.Value)
				},
			),
			rdsd.inclusionInstanceTags,
			rdsd.exclusionInstanceTags,
		) {
			continue
		}
		// build resource
		awsBaseDetails := discovery.AwsBaseDetails{
			AwsRegion:    rdsd.cfg.Region,
			AwsAccountId: awsAccountId,
			AwsArn:       aws.ToString(instance.DBInstanceArn),
		}
		tags := map[string]string{}
		for _, t := range instance.TagList {
			tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
		}
		rdsInstanceDetails := &discovery.AwsRdsInstanceDetails{
			AwsBaseDetails:       awsBaseDetails,
			Tags:                 tags,
			DbInstanceIdentifier: aws.ToString(instance.DBInstanceIdentifier),
			DbInstanceStatus:     aws.ToString(instance.DBInstanceStatus),
			Engine:               aws.ToString(instance.Engine),
			EngineVersion:        aws.ToString(instance.EngineVersion),
		}
		if instance.DBSubnetGroup != nil {
			rdsInstanceDetails.DBSubnetGroupName = aws.ToString(instance.DBSubnetGroup.DBSubnetGroupName)
			rdsInstanceDetails.VpcId = aws.ToString(instance.DBSubnetGroup.VpcId)
		} else {
			rdsInstanceDetails.DBSubnetGroupName = ""
			rdsInstanceDetails.VpcId = ""
		}
		if instance.Endpoint != nil {
			rdsInstanceDetails.EndpointAddress = aws.ToString(instance.Endpoint.Address)
			rdsInstanceDetails.EndpointPort = instance.Endpoint.Port
		} else {
			rdsInstanceDetails.EndpointAddress = ""
			rdsInstanceDetails.EndpointPort = -1
		}
		result.AddResources(discovery.Resource{
			ResourceType:          discovery.ResourceTypeAwsRdsInstance,
			AwsRdsInstanceDetails: rdsInstanceDetails,
		})
	}

	return result
}
