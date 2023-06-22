package discovery

const (
	// ResourceTypeAwsEc2Instance is the resource type for AWS EC2 instances.
	ResourceTypeAwsEc2Instance = "aws_ec2_instance"

	// ResourceTypeAwsEcsCluster is the resource type for AWS ECS clusters.
	ResourceTypeAwsEcsCluster = "aws_ecs_cluster"

	// ResourceTypeAwsRdsInstnace is the resource type for AWS RDS instances.
	ResourceTypeAwsRdsInstance = "aws_rds_instance"

	// ResourceTypeAwsSsmTarget is the resource type for AWS SSM targets.
	ResourceTypeAwsSsmTarget = "aws_ssm_target"

	// ResourceTypeNetworkHttpServer is the resource type for network-reachable HTTP servers.
	ResourceTypeNetworkHttpServer = "network_http_server"

	// ResourceTypeNetworkHttpsServer is the resource type for network-reachable HTTPS servers.
	ResourceTypeNetworkHttpsServer = "network_https_server"

	// ResourceTypeNetworkMysqlServer is the resource type for network-reachable MySQL servers.
	ResourceTypeNetworkMysqlServer = "network_mysql_server"

	// ResourceTypeNetworkPostgresqlServer is the resource type for network-reachable PostgreSQL servers.
	ResourceTypeNetworkPostgresqlServer = "network_postgresql_server"

	// ResourceTypeNetworkSshServer is the resource type for network-reachable SSH servers.
	ResourceTypeNetworkSshServer = "network_ssh_server"
)

// AwsBaseDetails represents the details of a discovered generic AWS resource.
type AwsBaseDetails struct {
	AwsAccountId string `json:"aws_account_id"`
	AwsRegion    string `json:"aws_region"`
	AwsArn       string `json:"aws_arn"`
}

// NetworkBaseDetails represents the details of a discovered generic service on the network.
type NetworkBaseDetails struct {
	Addresses []string `json:"addresses"`
	HostNames []string `json:"hostnames,omitempty"`
	Port      string   `json:"port"`
}

// AwsEc2InstanceDetails represents the details of a discovered AWS EC2 instance.
type AwsEc2InstanceDetails struct {
	AwsBaseDetails // extends

	Tags map[string]string `json:"tags"`

	InstanceId       string `json:"instance_id"`
	ImageId          string `json:"ami_id"`
	VpcId            string `json:"vpc_id"`
	SubnetId         string `json:"subnet_id"`
	PrivateDnsName   string `json:"private_dns_name"`
	PrivateIpAddress string `json:"private_ip_address"`
	PublicDnsName    string `json:"public_dns_name"`
	PublicIpAddress  string `json:"public_ip_address"`
	InstanceType     string `json:"instance_type"`

	// add any new fields as needed here
}

// AwsEcsClusterDetails represents the details of a discovered AWS ECS cluster.
type AwsEcsClusterDetails struct {
	AwsBaseDetails // extends

	Tags map[string]string `json:"tags"`

	ClusterName string   `json:"cluster_name"`
	Services    []string `json:"services"`
	Tasks       []string `json:"tasks"`
	Containers  []string `json:"containers"`

	// add any new fields as needed here
}

// AwsRdsInstanceDetails represents the details of a discovered AWS RDS instance.
type AwsRdsInstanceDetails struct {
	AwsBaseDetails // extends

	Tags map[string]string `json:"tags"`

	DBInstanceIdentifier string `json:"db_instance_identifier"`
	Engine               string `json:"engine"`
	EngineVersion        string `json:"engine_version"`
	VpcId                string `json:"vpc_id"`
	DBSubnetGroupName    string `json:"db_subnet_group_name"`
	EndpointAddress      string `json:"endpoint_address"`
	EndpointPort         int32  `json:"endpoint_port"`

	// add any new fields as needed here
}

// AwsSsmTargetDetails represents the details of a discovered AWS SSM target.
type AwsSsmTargetDetails struct {
	AwsBaseDetails // extends

	InstanceId string `json:"instance_id"`

	// add any new fields as needed here
}

// NetworkHttpServerDetails represents the details
// of a discovered HTTP server on the network.
type NetworkHttpServerDetails struct {
	NetworkBaseDetails // extends

	// add any new fields as needed here
}

// NetworkHttpsServerDetails represents the details
// of a discovered HTTPS server on the network.
type NetworkHttpsServerDetails struct {
	NetworkBaseDetails // extends

	// add any new fields as needed here
}

// NetworkMysqlServerDetails represents the details
// of a discovered MySQL server on the network.
type NetworkMysqlServerDetails struct {
	NetworkBaseDetails // extends

	// add any new fields as needed here
}

// NetworkPostgresqlServerDetails represents the details
// of a discovered PostgreSQL server on the network.
type NetworkPostgresqlServerDetails struct {
	NetworkBaseDetails // extends

	// add any new fields as needed here
}

// NetworkSshServerDetails represents the details
// of a discovered SSH server on the network.
type NetworkSshServerDetails struct {
	NetworkBaseDetails // extends

	// add any new fields as needed here
}

// Resource represents a generic discovered resource.
type Resource struct {
	ResourceType string `json:"resource_type"`

	AwsEc2InstanceDetails          *AwsEc2InstanceDetails          `json:"aws_ec2_instance_details,omitempty"`
	AwsEcsClusterDetails           *AwsEcsClusterDetails           `json:"aws_ecs_cluster_details,omitempty"`
	AwsRdsInstanceDetails          *AwsRdsInstanceDetails          `json:"aws_rds_instance_details,omitempty"`
	AwsSsmTargetDetails            *AwsSsmTargetDetails            `json:"aws_ssm_target_details,omitempty"`
	NetworkHttpServerDetails       *NetworkHttpServerDetails       `json:"network_http_server_details,omitempty"`
	NetworkHttpsServerDetails      *NetworkHttpsServerDetails      `json:"network_https_server_details,omitempty"`
	NetworkMysqlServerDetails      *NetworkMysqlServerDetails      `json:"network_mysql_server_details,omitempty"`
	NetworkPostgresqlServerDetails *NetworkPostgresqlServerDetails `json:"network_postgresql_server_details,omitempty"`
	NetworkSshServerDetails        *NetworkSshServerDetails        `json:"network_ssh_server_details,omitempty"`

	// add any new resource details here
}
