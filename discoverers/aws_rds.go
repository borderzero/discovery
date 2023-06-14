package discoverers

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/borderzero/discovery"
)

// AwsRdsDiscoverer represents a discoverer for AWS RDS resources.
type AwsRdsDiscoverer struct {
	cfg aws.Config
}

// ensure AwsRdsDiscoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*AwsRdsDiscoverer)(nil)

// AwsRdsDiscovererOption is an input option for the AwsRdsDiscoverer constructor.
type AwsRdsDiscovererOption func(*AwsRdsDiscoverer)

// NewAwsRdsDiscoverer returns a new AwsRdsDiscoverer, initialized with the given options.
func NewAwsRdsDiscoverer(cfg aws.Config, opts ...AwsRdsDiscovererOption) *AwsRdsDiscoverer {
	rdsd := &AwsRdsDiscoverer{cfg: cfg}
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

	// get caller identity
	gciCtx, gciCtxCancel := context.WithTimeout(ctx, time.Second*2)
	defer gciCtxCancel()
	stsClient := sts.NewFromConfig(rdsd.cfg)
	getCallerIdentityOutput, err := stsClient.GetCallerIdentity(gciCtx, &sts.GetCallerIdentityInput{})
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to get caller identity via sts: %v", err))
		return
	}
	awsAccountId := aws.ToString(getCallerIdentityOutput.Account)

	// describe rds instances
	rdsClient := rds.NewFromConfig(rdsd.cfg)
	// TODO: new context with timeout for describe instances
	describeDBInstancesOutput, err := rdsClient.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{})
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to describe rds instances: %v", err))
		return
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
		result.Resources = append(result.Resources, discovery.Resource{
			ResourceType:          discovery.ResourceTypeAwsRdsInstance,
			AwsRdsInstanceDetails: rdsInstanceDetails,
		})
	}
}
