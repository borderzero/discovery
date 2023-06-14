package discoverers

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/borderzero/discovery"
)

// AwsRdsDiscoverer represents a discoverer for AWS RDS resources.
type AwsRdsDiscoverer struct {
	cfg          aws.Config
	awsAccountId string // FIXME: call aws sts get-caller-identity for this
}

// ensure AwsRdsDiscoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*AwsRdsDiscoverer)(nil)

// AwsRdsDiscovererOption is an input option for the AwsRdsDiscoverer constructor.
type AwsRdsDiscovererOption func(*AwsRdsDiscoverer)

// NewAwsRdsDiscoverer returns a new AwsRdsDiscoverer, initialized with the given options.
func NewAwsRdsDiscoverer(cfg aws.Config, awsAccountId string, opts ...AwsRdsDiscovererOption) *AwsRdsDiscoverer {
	rdsd := &AwsRdsDiscoverer{cfg: cfg, awsAccountId: awsAccountId}
	for _, opt := range opts {
		opt(rdsd)
	}
	return rdsd
}

// Discover runs the AwsRdsDiscoverer and closes the channels after a single run.
func (rdsd *AwsRdsDiscoverer) Discover(
	ctx context.Context,
	results chan<- *discovery.Result,
) {
	result := discovery.NewResult()
	defer func() {
		result.Metrics.EndedAt = time.Now()
		results <- result
		close(results)
	}()

	rdsClient := rds.NewFromConfig(rdsd.cfg)
	describeDBInstancesOutput, err := rdsClient.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{})
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("failed to describe rds instances: %w", err))
		return
	}

	for _, instance := range describeDBInstancesOutput.DBInstances {
		// ignore unavailable instances
		if instance.DBInstanceStatus == nil || aws.ToString(instance.DBInstanceStatus) != "available" {
			continue
		}
		// build resource
		awsBaseDetails := discovery.AwsBaseDetails{
			AwsRegion:    rdsd.cfg.Region,
			AwsAccountId: rdsd.awsAccountId,
			AwsArn:       aws.ToString(instance.DBInstanceArn),
		}
		rdsInstanceDetails := &discovery.AwsRdsInstanceDetails{
			AwsBaseDetails:       awsBaseDetails,
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
		result.Resources = append(result.Resources, discovery.Resource{
			ResourceType:          discovery.ResourceTypeAwsRdsInstance,
			AwsRdsInstanceDetails: rdsInstanceDetails,
		})
	}
}
