package azurewrapper

import (
	"context"
	"fmt"

	uuid "github.com/kthomas/go.uuid"

	"github.com/Azure/azure-sdk-for-go/services/containerinstance/mgmt/2018-10-01/containerinstance"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-12-01/network"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
)

// NewContainerGroupsClient is creating a container group client
func NewContainerGroupsClient(subscriptionID string) (containerinstance.ContainerGroupsClient, error) {
	client := containerinstance.NewContainerGroupsClient(subscriptionID)
	if auth, err := GetAuthorizer(); err == nil {
		client.Authorizer = *auth
		return client, nil
	} else {
		return client, err
	}
}

// NewLoadBalancerClient is creating a load balancer client
func NewLoadBalancerClient(subscriptionID string) (network.LoadBalancersClient, error) {
	client := network.NewLoadBalancersClient(subscriptionID)
	if auth, err := GetAuthorizer(); err == nil {
		client.Authorizer = *auth
		return client, nil
	} else {
		return client, err
	}
}

// NewAuthorizer initializes a new authorizer from the configured environment
func newAuthorizer() (autorest.Authorizer, error) {
	// FIXME-- pass args in
	return auth.NewAuthorizerFromEnvironment()
}

// GetAuthorizer initializes new authorizer or returns existing
func GetAuthorizer() (*autorest.Authorizer, error) {
	authorizer, err := newAuthorizer()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve Azure authorizer; %s", err.Error())
	}
	return &authorizer, nil
}

// NewResourceGroupsClient initializes and returns an instance of the resource groups API client
func NewResourceGroupsClient(subscriptionID string) (resources.GroupsClient, error) {
	client := resources.NewGroupsClient(subscriptionID)
	if auth, err := GetAuthorizer(); err == nil {
		client.Authorizer = *auth
		return client, nil
	} else {
		return client, err
	}
}

// NewVirtualNetworksClient initializes and returns an instance of the Azure vnet API client
func NewVirtualNetworksClient(subscriptionID string) (network.VirtualNetworksClient, error) {
	client := network.NewVirtualNetworksClient(subscriptionID)
	if auth, err := GetAuthorizer(); err == nil {
		client.Authorizer = *auth
		return client, nil
	} else {
		return client, err
	}
}

// StartContainer starts a new node in network
func StartContainer(ctx context.Context,
	subscriptionID string,
	region string,
	resourceGroupName string,
	image *string,
	virtualNetworkID *string,
	cpu, memory *int64,
	entrypoint []*string,
	securityGroupIds []string,
	subnetIds []string,
	environment map[string]interface{},
	security map[string]interface{},
) (ids []string, err error) {
	if image == nil {
		return ids, fmt.Errorf("Unable to start container in region: %s; container can only be started with a valid image or task definition", region)
	}

	if security != nil && len(security) == 0 {
		return ids, fmt.Errorf("Unable to start container w/o security config")
	}

	env := make([]containerinstance.EnvironmentVariable, 0)
	for k := range environment {
		if val, valOk := environment[k].(string); valOk {
			env = append(env, containerinstance.EnvironmentVariable{
				Name:  to.StringPtr(k),
				Value: to.StringPtr(val),
			})
		}
	}

	// var healthCheck *ecs.HealthCheck
	portMappings := make([]containerinstance.Port, 0)
	containerPortMappings := make([]containerinstance.ContainerPort, 0)

	if security != nil {
		if ingress, ingressOk := security["ingress"]; ingressOk {
			switch ingress.(type) {
			case map[string]interface{}:
				ingressCfg := ingress.(map[string]interface{})
				for cidr := range ingressCfg {
					if tcp, tcpOk := ingressCfg[cidr].(map[string]interface{})["tcp"].([]interface{}); tcpOk {
						for i := range tcp {
							port := int32(tcp[i].(float64))
							portMappings = append(portMappings, containerinstance.Port{
								Port:     &port,
								Protocol: containerinstance.TCP,
							})
							containerPortMappings = append(containerPortMappings, containerinstance.ContainerPort{
								Port: &port,
							})
						}
					}

					if udp, udpOk := ingressCfg[cidr].(map[string]interface{})["udp"].([]interface{}); udpOk {
						for i := range udp {
							port := int32(udp[i].(float64))
							portMappings = append(portMappings, containerinstance.Port{
								Port:     &port,
								Protocol: containerinstance.UDP,
							})
							containerPortMappings = append(containerPortMappings, containerinstance.ContainerPort{
								Port: &port,
							})
						}
					}
				}
			}
		}
	}

	containerGroupName, _ := uuid.NewV4()
	containerName, _ := uuid.NewV4()
	cgClient, err := NewContainerGroupsClient(subscriptionID)
	if err != nil {
		return ids, fmt.Errorf("Unable to get container client: %s; ", err.Error())
	}

	future, err := cgClient.CreateOrUpdate(
		ctx,
		resourceGroupName,
		containerGroupName.String(),
		containerinstance.ContainerGroup{
			Name:     to.StringPtr(containerGroupName.String()),
			Location: &region,
			ContainerGroupProperties: &containerinstance.ContainerGroupProperties{
				IPAddress: &containerinstance.IPAddress{
					Type:  containerinstance.Public,
					Ports: &portMappings,
				},
				OsType: containerinstance.Linux,
				Containers: &[]containerinstance.Container{
					{
						Name: to.StringPtr(containerName.String()),
						ContainerProperties: &containerinstance.ContainerProperties{
							EnvironmentVariables: &env,
							Image:                image,
							Ports:                &containerPortMappings,
							Resources: &containerinstance.ResourceRequirements{
								Limits: &containerinstance.ResourceLimits{
									MemoryInGB: to.Float64Ptr(float64(*memory)),
									CPU:        to.Float64Ptr(float64(*cpu)),
								},
								Requests: &containerinstance.ResourceRequests{
									MemoryInGB: to.Float64Ptr(float64(*memory)),
									CPU:        to.Float64Ptr(float64(*cpu)),
								},
							},
						},
					},
				},
			},
		},
	)

	if err != nil {
		log.Warningf("failed to create container group; %s", err.Error())
		return []string{}, err
	}

	err = future.WaitForCompletionRef(ctx, cgClient.Client)
	if err != nil {
		log.Warningf("failed to create container group; %s", err.Error())
		return []string{}, err
	}

	containerGroup, err := future.Result(cgClient)
	if err != nil {
		log.Warningf("failed to create container group; %s", err.Error())
		return []string{}, err
	}

	return []string{*containerGroup.ID}, nil
}

// UpsertResourceGroup upserts a resource group for the given params
func UpsertResourceGroup(ctx context.Context, subscriptionID, region, name string) (*resources.Group, error) {
	gClient, err := NewResourceGroupsClient(subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to init resource groups client; %s", err.Error())
	}

	group := resources.Group{
		Location: to.StringPtr(region),
	}

	group, err = gClient.CreateOrUpdate(ctx, name, group)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert resource group; %s", err.Error())
	}

	return &group, nil
}

// UpsertVirtualNetwork upserts a resource group for the given params
func UpsertVirtualNetwork(ctx context.Context, subscriptionID, groupName, name, region string) (*network.VirtualNetwork, error) {
	vnetClient, _ := NewVirtualNetworksClient(subscriptionID)
	future, err := vnetClient.CreateOrUpdate(
		ctx,
		groupName,
		name,
		network.VirtualNetwork{
			Location: to.StringPtr(region),
			VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
				AddressSpace: &network.AddressSpace{
					AddressPrefixes: &[]string{"10.0.0.0/8"},
				},
				Subnets: &[]network.Subnet{
					{
						Name: to.StringPtr("subnet1Name"),
						SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
							AddressPrefix: to.StringPtr("10.0.0.0/16"),
						},
					},
					{
						Name: to.StringPtr("subnet2Name"),
						SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
							AddressPrefix: to.StringPtr("10.1.0.0/16"),
						},
					},
				},
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("cannot create virtual network: %v", err)
	}

	err = future.WaitForCompletionRef(ctx, vnetClient.Client)
	if err != nil {
		return nil, fmt.Errorf("cannot get the vnet create or update future response: %v", err)
	}

	vnet, err := future.Result(vnetClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create new virtual network; %s", err.Error())
	}

	return &vnet, nil
}

// CreateLoadBalancer creates load balancer for a group
func CreateLoadBalancer(ctx context.Context, lbName, location, pipName, groupName, subscriptionID string, security map[string]interface{}) (lb *network.LoadBalancer, err error) {
	if security != nil && len(security) == 0 {
		return lb, fmt.Errorf("Unable to start container w/o security config")
	}

	probeName := "probe"
	frontEndIPConfigName := "fip"
	backEndAddressPoolName := "backEndPool"
	idPrefix := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers", subscriptionID, groupName)

	pip, err := GetPublicIP(ctx, pipName, groupName, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get public IP address; %s", err.Error())
	}
	println(fmt.Sprintf("ip: %+v", pip))

	lbClient, err := NewLoadBalancerClient(subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create load balancer client; %s", err.Error())
	}
	println(fmt.Sprintf("client: %+v", lbClient))

	rules := make([]network.LoadBalancingRule, 0)
	inboundNatRules := make([]network.InboundNatRule, 0)
	// outboundRules := make([]network.OutboundRule, 0)
	var healthCheckPort int32
	// defaultRule := network.LoadBalancingRule{
	// 	Name: to.StringPtr("lbRule"),
	// 	LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
	// 		Protocol:             network.TransportProtocolTCP,
	// 		FrontendPort:         to.Int32Ptr(80),
	// 		BackendPort:          to.Int32Ptr(80),
	// 		IdleTimeoutInMinutes: to.Int32Ptr(4),
	// 		EnableFloatingIP:     to.BoolPtr(false),
	// 		LoadDistribution:     network.LoadDistributionDefault,
	// 		FrontendIPConfiguration: &network.SubResource{
	// 			ID: to.StringPtr(fmt.Sprintf("/%s/%s/frontendIPConfigurations/%s", idPrefix, lbName, frontEndIPConfigName)),
	// 		},
	// 		BackendAddressPool: &network.SubResource{
	// 			ID: to.StringPtr(fmt.Sprintf("/%s/%s/backendAddressPools/%s", idPrefix, lbName, backEndAddressPoolName)),
	// 		},
	// 		Probe: &network.SubResource{
	// 			ID: to.StringPtr(fmt.Sprintf("/%s/%s/probes/%s", idPrefix, lbName, probeName)),
	// 		},
	// 	},
	// }
	// rules = append(rules, defaultRule)

	// portMappings := make([]containerinstance.Port, 0)
	// containerPortMappings := make([]containerinstance.ContainerPort, 0)

	if security != nil {
		if ingress, ingressOk := security["ingress"]; ingressOk {
			switch ingress.(type) {
			case map[string]interface{}:
				ingressCfg := ingress.(map[string]interface{})
				for cidr := range ingressCfg {
					if tcp, tcpOk := ingressCfg[cidr].(map[string]interface{})["tcp"].([]interface{}); tcpOk {

						for x, i := range tcp {
							port := int32(i.(float64))
							if x == 0 {
								healthCheckPort = port
							}

							rule := network.LoadBalancingRule{
								Name: to.StringPtr(fmt.Sprintf("lbRuleTcp%d", x)),
								LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
									Protocol:             network.TransportProtocolTCP,
									FrontendPort:         to.Int32Ptr(port),
									BackendPort:          to.Int32Ptr(port),
									IdleTimeoutInMinutes: to.Int32Ptr(4),
									EnableFloatingIP:     to.BoolPtr(false),
									LoadDistribution:     network.LoadDistributionDefault,
									FrontendIPConfiguration: &network.SubResource{
										ID: to.StringPtr(fmt.Sprintf("/%s/%s/frontendIPConfigurations/%s", idPrefix, lbName, frontEndIPConfigName)),
									},
									BackendAddressPool: &network.SubResource{
										ID: to.StringPtr(fmt.Sprintf("/%s/%s/backendAddressPools/%s", idPrefix, lbName, backEndAddressPoolName)),
									},
									Probe: &network.SubResource{
										ID: to.StringPtr(fmt.Sprintf("/%s/%s/probes/%s", idPrefix, lbName, probeName)),
									},
								},
							}
							rules = append(rules, rule)

							// natRule := network.InboundNatRule{
							// 	Name: to.StringPtr(fmt.Sprintf("natRuleTcp%d", x)),
							// 	InboundNatRulePropertiesFormat: &network.InboundNatRulePropertiesFormat{
							// 		Protocol:             network.TransportProtocolTCP,
							// 		FrontendPort:         to.Int32Ptr(port),
							// 		BackendPort:          to.Int32Ptr(port),
							// 		EnableFloatingIP:     to.BoolPtr(false),
							// 		IdleTimeoutInMinutes: to.Int32Ptr(5),
							// 		FrontendIPConfiguration: &network.SubResource{
							// 			ID: to.StringPtr(fmt.Sprintf("/%s/%s/frontendIPConfigurations/%s", idPrefix, lbName, frontEndIPConfigName)),
							// 		},
							// 	},
							// }
							// inboundNatRules = append(inboundNatRules, natRule)

							// outboundRules = append()

						}
					}

					if udp, udpOk := ingressCfg[cidr].(map[string]interface{})["udp"].([]interface{}); udpOk {
						for i := range udp {
							port := int32(udp[i].(float64))

							rule := network.LoadBalancingRule{
								Name: to.StringPtr(fmt.Sprintf("lbRuleUdp%d", i)),
								LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
									Protocol:             network.TransportProtocolUDP,
									FrontendPort:         to.Int32Ptr(port),
									BackendPort:          to.Int32Ptr(port),
									IdleTimeoutInMinutes: to.Int32Ptr(4),
									EnableFloatingIP:     to.BoolPtr(false),
									LoadDistribution:     network.LoadDistributionDefault,
									FrontendIPConfiguration: &network.SubResource{
										ID: to.StringPtr(fmt.Sprintf("/%s/%s/frontendIPConfigurations/%s", idPrefix, lbName, frontEndIPConfigName)),
									},
									BackendAddressPool: &network.SubResource{
										ID: to.StringPtr(fmt.Sprintf("/%s/%s/backendAddressPools/%s", idPrefix, lbName, backEndAddressPoolName)),
									},
									Probe: &network.SubResource{
										ID: to.StringPtr(fmt.Sprintf("/%s/%s/probes/%s", idPrefix, lbName, probeName)),
									},
								},
							}
							rules = append(rules, rule)

							// natRule := network.InboundNatRule{
							// 	Name: to.StringPtr(fmt.Sprintf("natRuleUdp%d", i)),
							// 	InboundNatRulePropertiesFormat: &network.InboundNatRulePropertiesFormat{
							// 		Protocol:             network.TransportProtocolUDP,
							// 		FrontendPort:         to.Int32Ptr(port),
							// 		BackendPort:          to.Int32Ptr(port),
							// 		EnableFloatingIP:     to.BoolPtr(false),
							// 		IdleTimeoutInMinutes: to.Int32Ptr(5),
							// 		FrontendIPConfiguration: &network.SubResource{
							// 			ID: to.StringPtr(fmt.Sprintf("/%s/%s/frontendIPConfigurations/%s", idPrefix, lbName, frontEndIPConfigName)),
							// 		},
							// 	},
							// }
							// inboundNatRules = append(inboundNatRules, natRule)

						}
					}
				}
			}
		}
	}

	future, err := lbClient.CreateOrUpdate(ctx,
		groupName,
		lbName,
		network.LoadBalancer{
			Location: to.StringPtr(location),
			LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
				FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
					{
						Name: &frontEndIPConfigName,
						FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
							PrivateIPAllocationMethod: network.Dynamic,
							PublicIPAddress:           &pip,
						},
					},
				},
				BackendAddressPools: &[]network.BackendAddressPool{
					{
						Name: &backEndAddressPoolName,
					},
				},
				Probes: &[]network.Probe{
					{
						Name: &probeName,
						ProbePropertiesFormat: &network.ProbePropertiesFormat{
							Protocol:          network.ProbeProtocolTCP,
							Port:              to.Int32Ptr(healthCheckPort),
							IntervalInSeconds: to.Int32Ptr(30),
							NumberOfProbes:    to.Int32Ptr(2),
						},
					},
				},
				LoadBalancingRules: &rules,
				InboundNatRules:    &inboundNatRules,
				// OutboundRules:      &outboundRules,
			},
		})

	if err != nil {
		return lb, fmt.Errorf("cannot create load balancer: %v", err)
	}

	err = future.WaitForCompletionRef(ctx, lbClient.Client)
	if err != nil {
		return lb, fmt.Errorf("cannot get load balancer create or update future response: %v", err)
	}

	balancer, err := future.Result(lbClient)
	return &balancer, err
}

// NewIPClient creates public IP addresses client
func NewIPClient(subscriptionID string) (network.PublicIPAddressesClient, error) {
	client := network.NewPublicIPAddressesClient(subscriptionID)
	if auth, err := GetAuthorizer(); err == nil {
		client.Authorizer = *auth
		return client, nil
	} else {
		return client, err
	}
}

// GetPublicIP returns an existing public IP
func GetPublicIP(ctx context.Context, ipName, groupName, subscriptionID string) (network.PublicIPAddress, error) {
	ipClient, _ := NewIPClient(subscriptionID)
	return ipClient.Get(ctx, groupName, ipName, "")
}

// CreatePublicIP creates public IP address
func CreatePublicIP(ctx context.Context, ipName, location, groupName, subscriptionID string) (ip *network.PublicIPAddress, err error) {
	ipClient, err := NewIPClient(subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to init public IP client; %s", err.Error())
	}

	future, err := ipClient.CreateOrUpdate(
		ctx,
		groupName,
		ipName,
		network.PublicIPAddress{
			Name:     to.StringPtr(ipName),
			Location: to.StringPtr(location),
			PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
				PublicIPAddressVersion:   network.IPv4,
				PublicIPAllocationMethod: network.Static,
			},
		},
	)

	if err != nil {
		return ip, fmt.Errorf("cannot create public ip address: %v", err)
	}

	err = future.WaitForCompletionRef(ctx, ipClient.Client)
	if err != nil {
		return ip, fmt.Errorf("cannot get public ip address create or update future response: %v", err)
	}

	addr, err := future.Result(ipClient)
	return &addr, err
}
