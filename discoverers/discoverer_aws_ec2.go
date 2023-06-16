package discoverers

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/borderzero/discovery"
	"github.com/borderzero/discovery/lib/types/slice"
)

const (
	defaultEc2DescribeInstancesTimeout = time.Second * 5
)

var (
	defaultIncludedEc2InstanceStates = []types.InstanceStateName{
		types.InstanceStateNameRunning,
		types.InstanceStateNamePending,
	}
)

// AwsEc2Client represents an entity capable of acting as the aws ec2 API client.
type AwsEc2Client interface {
	DescribeInstances(context.Context, *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error)
}

// AwsEc2Discoverer represents a discoverer for AWS EC2 resources.
type AwsEc2Discoverer struct {
	awsEc2Client AwsEc2Client

	resourceLabelAwsRegion    string
	resourceLabelAwsAccountId string

	describeInstancesTimeout time.Duration

	includeInstanceStates []types.InstanceStateName
}

// ensure AwsEc2Discoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*AwsEc2Discoverer)(nil)

// NewEngine returns a new AwsEc2Discoverer, initialized with the given options.
func NewAwsEc2Discoverer(
	awsEc2Client AwsEc2Client,
	resourceLabelAwsRegion string,
	resourceLabelAwsAccountId string,
) *AwsEc2Discoverer {
	return &AwsEc2Discoverer{
		awsEc2Client:              awsEc2Client,
		resourceLabelAwsRegion:    resourceLabelAwsRegion,
		resourceLabelAwsAccountId: resourceLabelAwsAccountId,
		describeInstancesTimeout:  defaultEc2DescribeInstancesTimeout,
		includeInstanceStates:     defaultIncludedEc2InstanceStates,
	}
}

// Discover runs the AwsEc2Discoverer and closes the channels after a single run.
func (ec2d *AwsEc2Discoverer) Discover(ctx context.Context) *discovery.Result {
	result := discovery.NewResult()
	defer result.Done()

	// describe ec2 instances
	describeInstancesCtx, describeInstancesCtxCancel := context.WithTimeout(ctx, ec2d.describeInstancesTimeout)
	defer describeInstancesCtxCancel()
	describeInstancesOutput, err := ec2d.awsEc2Client.DescribeInstances(
		describeInstancesCtx,
		&ec2.DescribeInstancesInput{},
	)
	if err != nil {
		result.AddError(fmt.Errorf("failed to describe ec2 instances: %w", err))
		return result
	}

	// filter and build resources
	for _, reservation := range describeInstancesOutput.Reservations {
		for _, instance := range reservation.Instances {
			// ignore unavailable instances
			if instance.State == nil || !slice.Contains(ec2d.includeInstanceStates, instance.State.Name) {
				continue
			}
			// build resource
			instanceId := aws.ToString(instance.InstanceId)
			awsBaseDetails := discovery.AwsBaseDetails{
				AwsRegion:    ec2d.resourceLabelAwsRegion,
				AwsAccountId: ec2d.resourceLabelAwsAccountId,
				AwsArn: fmt.Sprintf(
					"arn:aws:ec2:%s:%s:instance/%s",
					ec2d.resourceLabelAwsRegion,
					ec2d.resourceLabelAwsAccountId,
					instanceId,
				),
			}
			tags := map[string]string{}
			for _, t := range instance.Tags {
				tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
			}
			ec2InstanceDetails := &discovery.AwsEc2InstanceDetails{
				AwsBaseDetails:   awsBaseDetails,
				Tags:             tags,
				InstanceId:       aws.ToString(instance.InstanceId),
				ImageId:          aws.ToString(instance.ImageId),
				VpcId:            aws.ToString(instance.VpcId),
				SubnetId:         aws.ToString(instance.SubnetId),
				PrivateDnsName:   aws.ToString(instance.PrivateDnsName),
				PrivateIpAddress: aws.ToString(instance.PrivateIpAddress),
				PublicDnsName:    aws.ToString(instance.PublicDnsName),
				PublicIpAddress:  aws.ToString(instance.PublicIpAddress),
				InstanceType:     string(instance.InstanceType),
			}
			result.AddResource(discovery.Resource{
				ResourceType:          discovery.ResourceTypeAwsEc2Instance,
				AwsEc2InstanceDetails: ec2InstanceDetails,
			})
		}
	}

	return result
}
