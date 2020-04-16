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
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
)

var (
	authorizer     autorest.Authorizer
	subscriptionID string
)

func main() {

	// if err := envy.Load(); err != nil {
	// 	println(fmt.Sprintf("env load error: %v", err.Error()))
	// 	return
	// }

	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Second)
	defer cancel()

	groupName := "skynet"

	_, err := UpsertResourceGroup(ctx, groupName, subscriptionID)
	if err != nil {
		println(fmt.Sprintf("cannot create group: %v", err.Error()))
	}

	vnet, err := UpsertVirtualNetwork(ctx, subscriptionID, groupName, "skynet-vpc", "eastus")
	if err != nil {
		panic(fmt.Sprintf("virtual network creation failed"))
	}

	println(fmt.Sprintf("vnet: %+v", vnet))

	image := to.StringPtr("provide/nats-server:latest")

	container, err := StartContainer(ctx, groupName, image, to.StringPtr(groupName),
		subscriptionID, to.Int64Ptr(2), to.Int64Ptr(4), []*string{}, []string{}, []string{"subnet1", "subnet2"},
		map[string]interface{}{}, "eastus", map[string]interface{}{})

	println(fmt.Sprintf("container: %+v", container))

	return
}

// NewContainerGroupsClient is creating a container group client
func NewContainerGroupsClient(subscriptionID string) (containerinstance.ContainerGroupsClient, error) {
	client := containerinstance.NewContainerGroupsClient(subscriptionID)
	if auth, err := GetAuthorizer(); err == nil {
		client.Authorizer = auth
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
func GetAuthorizer() (autorest.Authorizer, error) {
	if authorizer == nil {
		authorizer, err := newAuthorizer()
		return authorizer, err
	}
	return authorizer, nil
}

// NewResourceGroupsClient initializes and returns an instance of the resource groups API client
func NewResourceGroupsClient(subscriptionID string) (resources.GroupsClient, error) {
	client := resources.NewGroupsClient(subscriptionID)
	if auth, err := GetAuthorizer(); err == nil {
		client.Authorizer = auth
		return client, nil
	} else {
		return client, err
	}
}

// NewVirtualNetworksClient initializes and returns an instance of the Azure vnet API client
func NewVirtualNetworksClient(subscriptionID string) (network.VirtualNetworksClient, error) {
	client := network.NewVirtualNetworksClient(subscriptionID)
	if auth, err := GetAuthorizer(); err == nil {
		client.Authorizer = auth
		return client, nil
	} else {
		return client, err
	}
}

// StartContainer starts a new node in network
func StartContainer(ctx context.Context,
	resourceGroupName string,
	image *string,
	virtualNetworkID *string,
	subscriptionID string,
	cpu, memory *int64,
	entrypoint []*string,
	securityGroupIds []string,
	subnetIds []string,
	environment map[string]interface{},
	region string,
	security map[string]interface{}) (ids []string, err error) {
	if image == nil {
		return ids, fmt.Errorf("Unable to start container in region: %s; container can only be started with a valid image or task definition", region)
	}

	if security != nil && len(security) > 0 {
		return ids, fmt.Errorf("Unable to start container in region: %s; security configuration not yet supported when a task definition arn is provided as the target to be started", region)
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

	containerGroupName := uuid.NewV4()
	containerName := fmt.Sprintf("%s - %s", containerGroupName.String(), *image)

	cgClient, err := NewContainerGroupsClient(subscriptionID)
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
						Name: &containerName,
						ContainerProperties: &containerinstance.ContainerProperties{
							Ports: &containerPortMappings,
							Image: image,
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
func UpsertResourceGroup(ctx context.Context, name, region string) (*resources.Group, error) {
	gClient, err := NewResourceGroupsClient(region)
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
