package discoverers

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/borderzero/discovery"
	"github.com/borderzero/discovery/lib/types/slice"
)

const defaultRdsDescribeDBInstancesTimeout = time.Second * 5

var defaultIncludedRdsDBInstanceStatuses = []string{"available"}

// AwsRdsClient represents an entity capable of acting as the aws rds API client.
type AwsRdsClient interface {
	DescribeDBInstances(context.Context, *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error)
}

// AwsRdsDiscoverer represents a discoverer for AWS RDS resources.
type AwsRdsDiscoverer struct {
	awsRdsClient AwsRdsClient

	describeDBInstancesTimeout time.Duration

	includeDBInstanceStatuses []string
}

// ensure AwsRdsDiscoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*AwsRdsDiscoverer)(nil)

// NewAwsRdsDiscoverer returns a new AwsRdsDiscoverer, initialized with the given options.
func NewAwsRdsDiscoverer(awsRdsClient AwsRdsClient) *AwsRdsDiscoverer {
	return &AwsRdsDiscoverer{
		awsRdsClient:               awsRdsClient,
		describeDBInstancesTimeout: defaultRdsDescribeDBInstancesTimeout,
		includeDBInstanceStatuses:  defaultIncludedRdsDBInstanceStatuses,
	}
}

// Discover runs the AwsRdsDiscoverer and closes the channels after a single run.
func (rdsd *AwsRdsDiscoverer) Discover(ctx context.Context) *discovery.Result {
	result := discovery.NewResult()
	defer result.Done()

	// describe rds instances
	describeDBInstancesCtx, describeDBInstancesCtxCancel := context.WithTimeout(ctx, rdsd.describeDBInstancesTimeout)
	defer describeDBInstancesCtxCancel()
	describeDBInstancesOutput, err := rdsd.awsRdsClient.DescribeDBInstances(
		describeDBInstancesCtx,
		&rds.DescribeDBInstancesInput{},
	)
	if err != nil {
		result.AddError(fmt.Errorf("failed to list ecs clusters: %w", err))
		return result
	}

	// filter and build resources
	for _, instance := range describeDBInstancesOutput.DBInstances {
		// ignore unavailable instances
		if !slice.Contains(rdsd.includeDBInstanceStatuses, aws.ToString(instance.DBInstanceStatus)) {
			continue
		}
		// parse arn
		dbInstanceArnString := aws.ToString(instance.DBInstanceArn)
		dbInstanceArn, err := arn.Parse(dbInstanceArnString)
		if err != nil {
			result.AddError(fmt.Errorf("got an invalid rds db instance arn \"%s\"", dbInstanceArnString))
			continue
		}
		// build resource
		awsBaseDetails := discovery.AwsBaseDetails{
			AwsRegion:    dbInstanceArn.Region,
			AwsAccountId: dbInstanceArn.AccountID,
			AwsArn:       dbInstanceArnString,
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
		result.AddResource(discovery.Resource{
			ResourceType:          discovery.ResourceTypeAwsRdsInstance,
			AwsRdsInstanceDetails: rdsInstanceDetails,
		})
	}

	return result
}
