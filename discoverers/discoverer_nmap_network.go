package discoverers

import (
	"context"
	"fmt"
	"time"

	"github.com/Ullaakut/nmap"
	"github.com/borderzero/border0-go/lib/types/slice"
	"github.com/borderzero/discovery"
)

const (
	defaultNmapNetworkDiscovererDiscovererId = "nmap_network_discoverer"
	defaultNmapNetworkDiscovererScanTimeout  = time.Second * 120

	nmapDiscoveredPortServiceNameHttp       = "http"
	nmapDiscoveredPortServiceNameHttpProxy  = "http-proxy"
	nmapDiscoveredPortServiceNameHttps      = "https"
	nmapDiscoveredPortServiceNameSsh        = "ssh"
	nmapDiscoveredPortServiceNameMysql      = "mysql"
	nmapDiscoveredPortServiceNamePostgresql = "postgresql"
)

var (
	defaultNmapNetworkDiscovererTargets = []string{
		"192.168.1.0/24",
	}
	defaultNmapNetworkDiscovererPorts = []string{
		"22",   // default ssh port
		"80",   // default http port
		"443",  // default https port
		"3306", // default mysql port
		"5432", // default postgresql port
		"8080", // common http port
		"8443", // common https port
	}
)

// NmapNetworkDiscoverer represents a discoverer for network-reachable
// resources using a library that relies on the nmap binary being present.
// Note that running the NmapNetworkDiscoverer without the nmap binary being
// available will result in failure to discover resources with this Discoverer.
type NmapNetworkDiscoverer struct {
	discovererId string
	scanTimeout  time.Duration
	nmapOpts     []func(*nmap.Scanner)
	targets      []string
	ports        []string
}

// ensure NmapNetworkDiscoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*NmapNetworkDiscoverer)(nil)

// NmapNetworkDiscovererOption represents a configuration option for an NmapNetworkDiscoverer.
type NmapNetworkDiscovererOption func(*NmapNetworkDiscoverer)

// WithNmapNetworkDiscovererDiscovererId is the NmapNetworkDiscovererOption to set a non default discoverer id.
func WithNmapNetworkDiscovererDiscovererId(discovererId string) NmapNetworkDiscovererOption {
	return func(nd *NmapNetworkDiscoverer) {
		nd.discovererId = discovererId
	}
}

// WithNmapNetworkDiscovererScanTimeout is the NmapNetworkDiscovererOption
// to set a non default timeout for scanning the network.
func WithNmapNetworkDiscovererScanTimeout(timeout time.Duration) NmapNetworkDiscovererOption {
	return func(nd *NmapNetworkDiscoverer) {
		nd.scanTimeout = timeout
	}
}

// WithNmapNetworkDiscovererTargets is the NmapNetworkDiscovererOption
// to set non default targets (IPs or DNS names) for discovery.
func WithNmapNetworkDiscovererTargets(targets ...string) NmapNetworkDiscovererOption {
	return func(nd *NmapNetworkDiscoverer) {
		nd.targets = targets
	}
}

// WithNmapNetworkDiscovererPorts is the NmapNetworkDiscovererOption
// to set non default target ports for discovery.
func WithNmapNetworkDiscovererPorts(ports ...string) NmapNetworkDiscovererOption {
	return func(nd *NmapNetworkDiscoverer) {
		nd.ports = ports
	}
}

// WithNmapNetworkDiscovererMinScanDelay is the NmapNetworkDiscovererOption
// to set non default min delay for discovery (time between probes).
func WithNmapNetworkDiscovererMinScanDelay(delay time.Duration) NmapNetworkDiscovererOption {
	return func(nd *NmapNetworkDiscoverer) {
		nd.nmapOpts = append(nd.nmapOpts, nmap.WithScanDelay(delay))
	}
}

// WithNmapNetworkDiscovererMaxScanDelay is the NmapNetworkDiscovererOption
// to set non default max delay for discovery (time between probes).
func WithNmapNetworkDiscovererMaxScanDelay(delay time.Duration) NmapNetworkDiscovererOption {
	return func(nd *NmapNetworkDiscoverer) {
		nd.nmapOpts = append(nd.nmapOpts, nmap.WithMaxScanDelay(delay))
	}
}

// WithNmapNetworkDiscovererMinScanRate is the NmapNetworkDiscovererOption
// to set non default min rate for discovery (packets per second).
func WithNmapNetworkDiscovererMinScanRate(packetsPerSecond int) NmapNetworkDiscovererOption {
	return func(nd *NmapNetworkDiscoverer) {
		nd.nmapOpts = append(nd.nmapOpts, nmap.WithMinRate(packetsPerSecond))
	}
}

// WithNmapNetworkDiscovererMaxScanRate is the NmapNetworkDiscovererOption
// to set non default max rate for discovery (packets per second).
func WithNmapNetworkDiscovererMaxScanRate(packetsPerSecond int) NmapNetworkDiscovererOption {
	return func(nd *NmapNetworkDiscoverer) {
		nd.nmapOpts = append(nd.nmapOpts, nmap.WithMaxRate(packetsPerSecond))
	}
}

// NewNmapNetworkDiscoverer returns a new NmapNetworkDiscoverer, initialized with the given options.
func NewNmapNetworkDiscoverer(opts ...NmapNetworkDiscovererOption) *NmapNetworkDiscoverer {
	nd := &NmapNetworkDiscoverer{
		discovererId: defaultNmapNetworkDiscovererDiscovererId,
		scanTimeout:  defaultNmapNetworkDiscovererScanTimeout,
		targets:      defaultNmapNetworkDiscovererTargets,
		ports:        defaultNmapNetworkDiscovererPorts,
	}
	for _, opt := range opts {
		opt(nd)
	}
	return nd
}

// Discover runs the NmapNetworkDiscoverer.
func (nd *NmapNetworkDiscoverer) Discover(ctx context.Context) *discovery.Result {
	result := discovery.NewResult(nd.discovererId)
	defer result.Done()

	scanCtx, cancel := context.WithTimeout(ctx, nd.scanTimeout)
	defer cancel()

	// build options
	opts := append(
		[]func(*nmap.Scanner){
			nmap.WithContext(scanCtx),
			nmap.WithTargets(nd.targets...),
			nmap.WithPorts(nd.ports...),
		},
		nd.nmapOpts...,
	)

	// equivalent to `nmap -p ${PORTS} ${TARGETS[@]} ${OPT_FLAGS[@]}`
	scanner, err := nmap.NewScanner(opts...)
	if err != nil {
		result.AddErrorf("unable to create nmap scanner: %v", err)
		return result
	}

	// FIXME: support warnings in discoverer (second return argument of scanner.Run())?
	scannerResults, _, err := scanner.Run()
	if err != nil {
		result.AddErrorf("unable to run nmap scan: %v", err)
		return result
	}

	// process results
	for _, host := range scannerResults.Hosts {
		// filter out hosts with no ports or no addresses
		if len(host.Ports) == 0 || len(host.Addresses) == 0 {
			continue
		}
		for _, port := range host.Ports {
			// filter out closed ports
			if port.State.String() == "closed" || port.State.String() == "filtered" {
				continue
			}
			// filter out ports that are not TCP ports
			// as they are currently useless...
			if port.Protocol != "tcp" {
				continue
			}

			ipAddress := ""
			if len(host.Addresses) > 0 {
				ipAddress = host.Addresses[0].Addr
			}

			networkBaseDetails := discovery.NetworkBaseDetails{
				HostNames: slice.Transform(host.Hostnames, func(h nmap.Hostname) string { return h.String() }),
				IpAddress: ipAddress,
				Port:      fmt.Sprintf("%d", port.ID),
			}

			// http (or http-proxy)
			if port.Service.Name == nmapDiscoveredPortServiceNameHttp ||
				port.Service.Name == nmapDiscoveredPortServiceNameHttpProxy {
				result.AddResources(discovery.Resource{
					ResourceType: discovery.ResourceTypeNetworkHttpServer,
					NetworkHttpServerDetails: &discovery.NetworkHttpServerDetails{
						NetworkBaseDetails: networkBaseDetails,
					},
				})
				continue
			}

			// https
			if port.Service.Name == nmapDiscoveredPortServiceNameHttps {
				result.AddResources(discovery.Resource{
					ResourceType: discovery.ResourceTypeNetworkHttpsServer,
					NetworkHttpsServerDetails: &discovery.NetworkHttpsServerDetails{
						NetworkBaseDetails: networkBaseDetails,
					},
				})
				continue
			}

			// mysql
			if port.Service.Name == nmapDiscoveredPortServiceNameMysql {
				result.AddResources(discovery.Resource{
					ResourceType: discovery.ResourceTypeNetworkMysqlServer,
					NetworkMysqlServerDetails: &discovery.NetworkMysqlServerDetails{
						NetworkBaseDetails: networkBaseDetails,
					},
				})
				continue
			}

			// postgresql
			if port.Service.Name == nmapDiscoveredPortServiceNamePostgresql {
				result.AddResources(discovery.Resource{
					ResourceType: discovery.ResourceTypeNetworkPostgresqlServer,
					NetworkPostgresqlServerDetails: &discovery.NetworkPostgresqlServerDetails{
						NetworkBaseDetails: networkBaseDetails,
					},
				})
				continue
			}

			// ssh
			if port.Service.Name == nmapDiscoveredPortServiceNameSsh {
				result.AddResources(discovery.Resource{
					ResourceType: discovery.ResourceTypeNetworkSshServer,
					NetworkSshServerDetails: &discovery.NetworkSshServerDetails{
						NetworkBaseDetails: networkBaseDetails,
					},
				})
				continue
			}
		}
	}

	return result
}
