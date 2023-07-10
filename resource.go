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

	// ResourceTypeKubernetesService is the resource type for kubernetes services.
	ResourceTypeKubernetesService = "kubernetes_service"

	// ResourceTypeLocalDockerContainer is the resource type for containers managed by the local Docker daemon.
	ResourceTypeLocalDockerContainer = "local_docker_container"

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

	// Ec2InstanceSsmStatusOnline represents the SSM status of an EC2 instance that is associated and online.
	Ec2InstanceSsmStatusOnline = "online"

	// Ec2InstanceSsmStatusOffline represents the SSM status of an EC2 instance that is associated and offline.
	Ec2InstanceSsmStatusOffline = "offline"

	// Ec2InstanceSsmStatusNotChecked represents the SSM status of an EC2 instance that is not checked.
	Ec2InstanceSsmStatusNotChecked = "not_checked"

	// Ec2InstanceSsmStatusNotAssociated represents the SSM status of an EC2 instance that is not associated.
	Ec2InstanceSsmStatusNotAssociated = "not_associated"
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
	AvailabilityZone string `json:"availability_zone"`
	PrivateDnsName   string `json:"private_dns_name"`
	PrivateIpAddress string `json:"private_ip_address"`
	PublicDnsName    string `json:"public_dns_name"`
	PublicIpAddress  string `json:"public_ip_address"`
	InstanceType     string `json:"instance_type"`
	InstanceState    string `json:"instance_state"`

	InstanceSsmStatus string `json:"ssm_status"`

	// add any new fields as needed here
}

// AwsEcsClusterDetails represents the details of a discovered AWS ECS cluster.
type AwsEcsClusterDetails struct {
	AwsBaseDetails // extends

	Tags map[string]string `json:"tags"`

	ClusterName   string   `json:"cluster_name"`
	ClusterStatus string   `json:"cluster_status"`
	Services      []string `json:"services"`
	Tasks         []string `json:"tasks"`
	Containers    []string `json:"containers"`

	// add any new fields as needed here
}

// AwsRdsInstanceDetails represents the details of a discovered AWS RDS instance.
type AwsRdsInstanceDetails struct {
	AwsBaseDetails // extends

	Tags map[string]string `json:"tags"`

	DbInstanceIdentifier string `json:"db_instance_identifier"`
	DbInstanceStatus     string `json:"db_instance_status"`
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
	PingStatus string `json:"ping_status"`

	// add any new fields as needed here
}

// KubernetesServicePort represents the details of a port for a kubernetes service.
type KubernetesServicePort struct {
	Name        string  `json:"name,omitempty"`
	Protocol    string  `json:"protocol,omitempty"`
	AppProtocol *string `json:"app_protocol,omitempty"`
	Port        int32   `json:"port"`
	TargetPort  string  `json:"target_port,omitempty"`
	NodePort    int32   `json:"node_port,omitempty"`
}

// KubernetesServiceDetails represents the details of a discovered kubernetes service.
type KubernetesServiceDetails struct {
	Namespace      string                  `json:"namespace"`
	Name           string                  `json:"name"`
	Uid            string                  `json:"uid"`
	ServiceType    string                  `json:"service_type"`
	ExternalName   string                  `json:"external_name,omitempty"`
	LoadBalancerIp string                  `json:"load_balancer_ip,omitempty"`
	ClusterIp      string                  `json:"cluster_ip"`
	ClusterIps     []string                `json:"cluster_ips"`
	Ports          []KubernetesServicePort `json:"ports"`
	Labels         map[string]string       `json:"labels"`
	Annotations    map[string]string       `json:"annotations"`

	// add any new fields as needed here
}

// LocalDockerContainerDetails represents the details of a
// discovered container managed by the local Docker daemon.
type LocalDockerContainerDetails struct {
	ContainerId  string            `json:"container_id"`
	Status       string            `json:"status"`
	Image        string            `json:"image"`
	Names        []string          `json:"names"`
	PortBindings map[string]string `json:"port_bindings"`
	Labels       map[string]string `json:"labels"`

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
	KubernetesServiceDetails       *KubernetesServiceDetails       `json:"kubernetes_service_details,omitempty"`
	LocalDockerContainerDetails    *LocalDockerContainerDetails    `json:"local_docker_container_details,omitempty"`
	NetworkHttpServerDetails       *NetworkHttpServerDetails       `json:"network_http_server_details,omitempty"`
	NetworkHttpsServerDetails      *NetworkHttpsServerDetails      `json:"network_https_server_details,omitempty"`
	NetworkMysqlServerDetails      *NetworkMysqlServerDetails      `json:"network_mysql_server_details,omitempty"`
	NetworkPostgresqlServerDetails *NetworkPostgresqlServerDetails `json:"network_postgresql_server_details,omitempty"`
	NetworkSshServerDetails        *NetworkSshServerDetails        `json:"network_ssh_server_details,omitempty"`

	// add any new resource details here
}
