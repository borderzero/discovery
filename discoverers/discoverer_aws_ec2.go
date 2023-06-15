package discoverers

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/borderzero/discovery"
	"golang.org/x/exp/slices"
)

// AwsEc2Discoverer represents a discoverer for AWS EC2 resources.
type AwsEc2Discoverer struct {
	cfg aws.Config
}

// ensure AwsEc2Discoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*AwsEc2Discoverer)(nil)

// NewEngine returns a new AwsEc2Discoverer, initialized with the given options.
func NewAwsEc2Discoverer(cfg aws.Config) *AwsEc2Discoverer {
	return &AwsEc2Discoverer{cfg: cfg}
}

// Discover runs the AwsEc2Discoverer and closes the channels after a single run.
func (ec2d *AwsEc2Discoverer) Discover(ctx context.Context) *discovery.Result {
	result := discovery.NewResult()
	defer result.Done()

	// get caller identity
	gciCtx, gciCtxCancel := context.WithTimeout(ctx, time.Second*2)
	defer gciCtxCancel()
	stsClient := sts.NewFromConfig(ec2d.cfg)
	getCallerIdentityOutput, err := stsClient.GetCallerIdentity(gciCtx, &sts.GetCallerIdentityInput{})
	if err != nil {
		result.AddError(fmt.Errorf("failed to get caller identity via sts: %w", err))
		return result
	}
	awsAccountId := aws.ToString(getCallerIdentityOutput.Account)

	// describe ec2 instances
	ec2Client := ec2.NewFromConfig(ec2d.cfg)
	// TODO: new context with timeout for describe instances
	describeInstancesOutput, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
	if err != nil {
		result.AddError(fmt.Errorf("failed to describe ec2 instances: %w", err))
		return result
	}

	// filter and build resources
	for _, reservation := range describeInstancesOutput.Reservations {
		for _, instance := range reservation.Instances {
			// ignore unavailable instances
			if instance.State == nil ||
				!slices.Contains([]types.InstanceStateName{types.InstanceStateNameRunning, types.InstanceStateNamePending}, instance.State.Name) {
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
