package discoverers

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/borderzero/discovery"
	"github.com/borderzero/discovery/utils"
)

const (
	defaultAwsRdsDiscovererDiscovererId        = "aws_rds_discoverer"
	defaultAwsRdsDiscovererGetAccountIdTimeout = time.Second * 10
)

// AwsRdsDiscoverer represents a discoverer for AWS RDS resources.
type AwsRdsDiscoverer struct {
	cfg aws.Config

	discovererId        string
	getAccountIdTimeout time.Duration
}

// ensure AwsRdsDiscoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*AwsRdsDiscoverer)(nil)

// AwsRdsDiscovererOption represents a configuration option for an AwsRdsDiscoverer.
type AwsRdsDiscovererOption func(*AwsRdsDiscoverer)

// WithAwsEcsDiscovererDiscovererId is the AwsRdsDiscovererOption to set a non default discoverer id.
func WithAwsRdsDiscovererDiscovererId(discovererId string) AwsRdsDiscovererOption {
	return func(rdsd *AwsRdsDiscoverer) {
		rdsd.discovererId = discovererId
	}
}

// WithAwsRdsDiscovererGetAccountIdTimeout is the AwsRdsDiscovererOption
// to set a non default timeout for getting the aws account id.
func WithAwsRdsDiscovererGetAccountIdTimeout(timeout time.Duration) AwsRdsDiscovererOption {
	return func(rdsd *AwsRdsDiscoverer) {
		rdsd.getAccountIdTimeout = timeout
	}
}

// NewAwsRdsDiscoverer returns a new AwsRdsDiscoverer, initialized with the given options.
func NewAwsRdsDiscoverer(cfg aws.Config, opts ...AwsRdsDiscovererOption) *AwsRdsDiscoverer {
	rdsd := &AwsRdsDiscoverer{
		cfg: cfg,

		discovererId:        defaultAwsRdsDiscovererDiscovererId,
		getAccountIdTimeout: defaultAwsRdsDiscovererGetAccountIdTimeout,
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
		result.AddError(fmt.Errorf("failed to get AWS account ID from AWS configuration: %w", err))
		return result
	}

	// describe rds instances
	rdsClient := rds.NewFromConfig(rdsd.cfg)
	// TODO: new context with timeout for describe instances
	describeDBInstancesOutput, err := rdsClient.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{})
	if err != nil {
		result.AddError(fmt.Errorf("failed to describe rds instances: %w", err))
		return result
	}

	// filter and build resources
	for _, instance := range describeDBInstancesOutput.DBInstances {
		// ignore unavailable instances
		if instance.DBInstanceStatus == nil || aws.ToString(instance.DBInstanceStatus) != "available" {
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
			DBInstanceIdentifier: aws.ToString(instance.DBInstanceIdentifier),
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
