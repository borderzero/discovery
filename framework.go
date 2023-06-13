package discovery

import (
	"context"
)

const (
	// ResourceTypeAwsEc2Instance is the resource type for AWS EC2 instances.
	ResourceTypeAwsEc2Instance = "aws_ec2_instance"

	// ResourceTypeAwsEcsContainer is the resource type for AWS ECS containers.
	ResourceTypeAwsEcsContainer = "aws_ecs_container"

	// ResourceTypeAwsRdsInstnace is the resource type for AWS RDS instances.
	ResourceTypeAwsRdsInstance = "aws_rds_instance"
)

// AwsBaseDetails represents the details of a discovered generic AWS resource.
type AwsBaseDetails struct {
	AwsAccountId string `json:"aws_account_id"`
	AwsRegion    string `json:"aws_region"`
	AwsArn       string `json:"aws_arn"`
}

// AwsEc2InstanceDetails represents the details of a discovered AWS EC2 instance.
type AwsEc2InstanceDetails struct {
	AwsBaseDetails // extends

	InstanceId       string `json:"instance_id"`
	ImageId          string `json:"ami_id"`
	VpcId            string `json:"vpc_id"`
	SubnetId         string `json:"subnet_id"`
	PrivateDnsName   string `json:"private_dns_name"`
	PrivateIpAddress string `json:"private_ip_address"`
	PublicDnsName    string `json:"public_dns_name"`
	PublicIpAddress  string `json:"public_ip_address"`
	InstanceType     string `json:"instance_type"`

	// TODO: add any fields as needed here
}

// AwsEcsContainerDetails represents the details of a discovered AWS ECS container.
type AwsEcsContainerDetails struct {
	AwsBaseDetails // extends

	ClusterName   string `json:"cluser_name"`
	ServiceName   string `json:"service_name"`
	TaskName      string `json:"task_name"`
	ContainerName string `json:"container_name"`

	// TODO: add any fields as needed here
}

// AwsRdsInstanceDetails represents the details of a discovered AWS RDS instance.
type AwsRdsInstanceDetails struct {
	AwsBaseDetails // extends

	DBInstanceIdentifier string `json:"db_instance_identifier"`
	Engine               string `json:"engine"`
	VpcId                string `json:"vpc_id"`
	DBSubnetGroupName    string `json:"db_subnet_group_name"`
	EndpointAddress      string `json:"endpoint_address"`
	EndpointPort         string `json:"endpoint_port"`

	// TODO: add any fields as needed here
}

// Resource represents a potential Border0 target.
type Resource struct {
	ResourceType string `json:"resource_type"`

	AwsEc2InstanceDetails  *AwsEc2InstanceDetails  `json:"aws_ec2_instance_details,omitempty"`
	AwsEcsContainerDetails *AwsEcsContainerDetails `json:"aws_ecs_container_details,omitempty"`
	AwsRdsInstanceDetails  *AwsRdsInstanceDetails  `json:"aws_rds_instance_details,omitempty"`
}

// Discoverer represents an entity capable of discovering resources.
type Discoverer interface {
	Discover(context.Context, chan<- []Resource) error
}
