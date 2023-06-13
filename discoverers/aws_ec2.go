package discoverers

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/borderzero/discovery"
	"golang.org/x/exp/slices"
)

// AwsEc2Discoverer represents a discoverer for AWS EC2 resources.
type AwsEc2Discoverer struct {
	cfg          aws.Config
	awsAccountId string
}

// ensure AwsEc2Discoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*AwsEc2Discoverer)(nil)

// AwsEc2DiscovererOption is an input option for the AwsEc2Discoverer constructor.
type AwsEc2DiscovererOption func(*AwsEc2Discoverer)

// NewEngine returns a new AwsEc2Discoverer, initialized with the given options.
func NewAwsEc2Discoverer(cfg aws.Config, awsAccountId string, opts ...AwsEc2DiscovererOption) *AwsEc2Discoverer {
	ec2d := &AwsEc2Discoverer{cfg: cfg, awsAccountId: awsAccountId}
	for _, opt := range opts {
		opt(ec2d)
	}
	return ec2d
}

// Discover runs the AwsEc2Discoverer and closes the channels after a single run.
func (ec2d *AwsEc2Discoverer) Discover(
	ctx context.Context,
	resources chan<- []discovery.Resource,
	errors chan<- error,
) {
	// discover routines are in charge of
	// closing their channels when done
	defer func() {
		close(resources)
		close(errors)
	}()

	ec2Client := ec2.NewFromConfig(ec2d.cfg)

	describeInstancesOutput, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
	if err != nil {
		errors <- fmt.Errorf("failed to describe ec2 instances: %w", err)
		return
	}

	discoveredResources := []discovery.Resource{}
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
				AwsAccountId: ec2d.awsAccountId,
				AwsArn: fmt.Sprintf(
					"arn:aws:ec2:%s:%s:instance/%s",
					ec2d.cfg.Region,
					ec2d.awsAccountId,
					instanceId,
				),
			}
			ec2InstanceDetails := &discovery.AwsEc2InstanceDetails{
				AwsBaseDetails:   awsBaseDetails,
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
			discoveredResources = append(discoveredResources, discovery.Resource{
				ResourceType:          discovery.ResourceTypeAwsEc2Instance,
				AwsEc2InstanceDetails: ec2InstanceDetails,
			})
		}
	}

	resources <- discoveredResources
}
