package azurewrapper

import (
	"context"
	"fmt"
	"time"

	uuid "github.com/kthomas/go.uuid"

	"github.com/Azure/azure-sdk-for-go/services/containerinstance/mgmt/2018-10-01/containerinstance"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-12-01/network"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"

	provide "github.com/provideservices/provide-go"
)

// NewContainerGroupsClient is creating a container group client
func NewContainerGroupsClient(tc *provide.TargetCredentials) (containerinstance.ContainerGroupsClient, error) {
	client := containerinstance.NewContainerGroupsClient(*tc.AzureSubscriptionID)
	if auth, err := GetAuthorizer(tc); err == nil {
		client.Authorizer = *auth
		return client, nil
	} else {
		return client, err
	}
}

// NewContainerClient is creating a container group client
func NewContainerClient(tc *provide.TargetCredentials) (containerinstance.ContainerClient, error) {
	client := containerinstance.NewContainerClient(*tc.AzureSubscriptionID)
	if auth, err := GetAuthorizer(tc); err == nil {
		client.Authorizer = *auth
		return client, nil
	} else {
		return client, err
	}
}

// NewLoadBalancerClient is creating a load balancer client
func NewLoadBalancerClient(tc *provide.TargetCredentials) (network.LoadBalancersClient, error) {
	client := network.NewLoadBalancersClient(*tc.AzureSubscriptionID)
	if auth, err := GetAuthorizer(tc); err == nil {
		client.Authorizer = *auth
		return client, nil
	} else {
		return client, err
	}
}

// NewAuthorizer initializes a new authorizer from the configured environment
func newAuthorizer(tc *provide.TargetCredentials) (autorest.Authorizer, error) {
	// FIXME-- pass args in
	// return auth.NewAuthorizerFromEnvironment()

	//settings, err := auth.GetSettingsFromEnvironment()

	settings := auth.EnvironmentSettings{
		Values: map[string]string{},
	}
	settings.Values["AZURE_SUBSCRIPTION_ID"] = *tc.AzureSubscriptionID
	settings.Values["AZURE_TENANT_ID"] = *tc.AzureTenantID
	settings.Values["AZURE_CLIENT_ID"] = *tc.AzureClientID
	settings.Values["AZURE_CLIENT_SECRET"] = *tc.AzureClientSecret
	settings.Environment = azure.PublicCloud
	settings.Values["AZURE_AD_RESOURCE"] = settings.Environment.ResourceManagerEndpoint

	// if err != nil {
	// 	return nil, err
	// }
	return settings.GetAuthorizer()

}

// GetAuthorizer initializes new authorizer or returns existing
func GetAuthorizer(tc *provide.TargetCredentials) (*autorest.Authorizer, error) {
	authorizer, err := newAuthorizer(tc)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve Azure authorizer; %s", err.Error())
	}
	return &authorizer, nil
}

// NewResourceGroupsClient initializes and returns an instance of the resource groups API client
func NewResourceGroupsClient(tc *provide.TargetCredentials) (resources.GroupsClient, error) {
	client := resources.NewGroupsClient(*tc.AzureSubscriptionID)
	if auth, err := GetAuthorizer(tc); err == nil {
		client.Authorizer = *auth
		return client, nil
	} else {
		return client, err
	}
}

// NewVirtualNetworksClient initializes and returns an instance of the Azure vnet API client
func NewVirtualNetworksClient(tc *provide.TargetCredentials) (network.VirtualNetworksClient, error) {
	client := network.NewVirtualNetworksClient(*tc.AzureSubscriptionID)
	if auth, err := GetAuthorizer(tc); err == nil {
		client.Authorizer = *auth
		return client, nil
	} else {
		return client, err
	}
}

func ContainerLogs(ctx context.Context, tc *provide.TargetCredentials, resourceGroupName, containerGroupName, containerID string) (logs containerinstance.Logs, err error) {
	cClient, err := NewContainerClient(tc)
	if err != nil {
		return logs, fmt.Errorf("Unable to get container client: %s; ", err.Error())
	}

	logs, err = cClient.ListLogs(ctx, resourceGroupName, containerGroupName, containerID, to.Int32Ptr(100))
	if err != nil {
		return logs, fmt.Errorf("Unable to get container logs: %s; ", err.Error())
	}

	return logs, nil
}

// DeleteContainer deletes container by its ID
func DeleteContainer(ctx context.Context, tc *provide.TargetCredentials, resourceGroupName string, containerID string) (err error) {
	cgClient, err := NewContainerGroupsClient(tc)
	if err != nil {
		return fmt.Errorf("Unable to get container group client: %s; ", err.Error())
	}

	_, err = cgClient.Delete(ctx, resourceGroupName, containerID)
	if err != nil {
		return fmt.Errorf("Unable to delete container: %s; ", err.Error())
	}

	return nil
}

// AzureContainerCreateParams is a struct representing the params needed to start an Azure container.
type AzureContainerCreateParams struct {
	context           context.Context
	subscriptionID    string
	region            string
	resourceGroupName string
	image             *string
	virtualNetworkID  *string
	cpu               *int64
	memory            *int64
	entrypoint        []*string
	securityGroupIds  []string
	subnetIds         []string
	environment       map[string]interface{}
	security          map[string]interface{}
}

// AzureContainerCreateResult is a struct representing the response from Azure container creation.
type AzureContainerCreateResult struct {
	ids          []string
	containerIds []string
	err          error
}

// StartContainer starts a new node in network
func StartContainer(cp *provide.ContainerParams, tc *provide.TargetCredentials) (result *provide.ContainerCreateResult, err error) {
	if cp.Image == nil {
		return nil, fmt.Errorf("Unable to start container in region: %s; container can only be started with a valid image or task definition", cp.Region)
	}

	security := cp.Security
	if security != nil && len(security) == 0 {
		return nil, fmt.Errorf("Unable to start container w/o security config")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Second)
	defer cancel()
	region := cp.Region
	resourceGroupName := cp.ResourceGroupName
	// virtualNetworkID *string,
	cpu := cp.CPU
	memory := cp.Memory

	environment := cp.Environment

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
	// containerName := cp.Image //uuid.NewV4()
	cgClient, err := NewContainerGroupsClient(tc)
	if err != nil {
		log.Warningf("Unable to get container group client: %s; ", err.Error())
		return nil, err
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
						Name: to.StringPtr(containerGroupName.String()),
						ContainerProperties: &containerinstance.ContainerProperties{
							EnvironmentVariables: &env,
							Image:                cp.Image,
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
		return nil, err
	}

	err = future.WaitForCompletionRef(ctx, cgClient.Client)
	if err != nil {
		log.Warningf("failed to create container group; %s", err.Error())
		return nil, err
	}

	containerGroup, err := future.Result(cgClient)
	if err != nil {
		log.Warningf("failed to create container group; %s", err.Error())
		return nil, err
	}

	// containerProperties := *(containerGroup.Containers)
	interfaces := make([]*provide.NetworkInterface, 1)
	intf := provide.NetworkInterface{
		Host:        containerGroup.ContainerGroupProperties.IPAddress.Fqdn,
		IPv4:        containerGroup.ContainerGroupProperties.IPAddress.IP,
		IPv6:        nil,
		PrivateIPv4: nil,
		PrivateIPv6: nil,
	}
	interfaces[0] = &intf

	return &provide.ContainerCreateResult{ContainerIds: []string{*containerGroup.Name}, ContainerInterfaces: interfaces}, nil
	// return []string{*containerGroup.ID}, []string{*containerGroup.Name}, nil
}

// DeleteResourceGroup deletes resource group
func DeleteResourceGroup(ctx context.Context, tc *provide.TargetCredentials, name string) (result bool, err error) {
	gClient, err := NewResourceGroupsClient(tc)
	if err != nil {
		return false, fmt.Errorf("failed to init resource groups client; %s", err.Error())
	}
	future, err := gClient.Delete(ctx, name)
	if err != nil {
		return false, fmt.Errorf("failed to delete resource group; %s", err.Error())
	}

	err = future.WaitForCompletionRef(ctx, gClient.Client)
	if err != nil {
		log.Warningf("failed to delete resource group; %s", err.Error())
		return false, fmt.Errorf("failed to delete resource group; %s", err.Error())
	}
	res, err := future.Result(gClient)

	return res.HasHTTPStatus(200), nil
}

// UpsertResourceGroup upserts a resource group for the given params
func UpsertResourceGroup(ctx context.Context, tc *provide.TargetCredentials, region, name string) (*string, error) {
	gClient, err := NewResourceGroupsClient(tc)
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

	return group.ID, nil
}

// DeleteVirtuaNetwork deletes virtual network
func DeleteVirtuaNetwork(ctx context.Context, tc *provide.TargetCredentials, resourceGroupName, virtualNetworkName string) (result bool, err error) {
	vnetClient, err := NewVirtualNetworksClient(tc)
	if err != nil {
		return false, fmt.Errorf("failed to init virtual network; %s", err.Error())
	}
	future, err := vnetClient.Delete(ctx, resourceGroupName, virtualNetworkName)
	if err != nil {
		return false, fmt.Errorf("failed to delete virtual network; %s", err.Error())
	}

	err = future.WaitForCompletionRef(ctx, vnetClient.Client)
	if err != nil {
		log.Warningf("failed to delete rvirtual network; %s", err.Error())
		return false, fmt.Errorf("failed to delete resource group; %s", err.Error())
	}
	res, err := future.Result(vnetClient)

	return res.HasHTTPStatus(200), nil
}

// UpsertVirtualNetwork upserts a resource group for the given params
func UpsertVirtualNetwork(ctx context.Context, tc *provide.TargetCredentials, groupName, name, region string) (*network.VirtualNetwork, error) {
	vnetClient, _ := NewVirtualNetworksClient(tc)
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

// DeleteLoadBalancer deletes load balancer from azure
func DeleteLoadBalancer(ctx context.Context, lbName, groupName string, tc *provide.TargetCredentials) (result bool, err error) {
	lbClient, err := NewLoadBalancerClient(tc)
	if err != nil {
		return false, fmt.Errorf("failed to create load balancer client; %s", err.Error())
	}

	future, err := lbClient.Delete(ctx, groupName, lbName)
	if err != nil {
		return false, fmt.Errorf("cannot delete load balancer: %v", err)
	}

	err = future.WaitForCompletionRef(ctx, lbClient.Client)
	if err != nil {
		return false, fmt.Errorf("cannot get load balancer create or update future response: %v", err)
	}

	response, err := future.Result(lbClient)
	return response.HasHTTPStatus(200), err
}

// CreateLoadBalancer creates load balancer for a group
func CreateLoadBalancer(ctx context.Context, lbName, location, pipName, groupName string, tc *provide.TargetCredentials, security map[string]interface{}) (lb *network.LoadBalancer, err error) {
	if security != nil && len(security) == 0 {
		return lb, fmt.Errorf("Unable to start container w/o security config")
	}

	probeName := "probe"
	frontEndIPConfigName := "fip"
	backEndAddressPoolName := "backEndPool"
	idPrefix := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers", *tc.AzureSubscriptionID, groupName)

	pip, err := GetPublicIP(ctx, pipName, groupName, tc)
	if err != nil {
		return nil, fmt.Errorf("failed to get public IP address; %s", err.Error())
	}
	println(fmt.Sprintf("ip: %+v", pip))

	lbClient, err := NewLoadBalancerClient(tc)
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
func NewIPClient(tc *provide.TargetCredentials) (network.PublicIPAddressesClient, error) {
	client := network.NewPublicIPAddressesClient(*tc.AzureSubscriptionID)
	if auth, err := GetAuthorizer(tc); err == nil {
		client.Authorizer = *auth
		return client, nil
	} else {
		return client, err
	}
}

// GetPublicIP returns an existing public IP
func GetPublicIP(ctx context.Context, ipName, groupName string, tc *provide.TargetCredentials) (network.PublicIPAddress, error) {
	ipClient, _ := NewIPClient(tc)
	return ipClient.Get(ctx, groupName, ipName, "")
}

// CreatePublicIP creates public IP address
func CreatePublicIP(ctx context.Context, ipName, location, groupName string, tc *provide.TargetCredentials) (ip *network.PublicIPAddress, err error) {
	ipClient, err := NewIPClient(tc)
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
