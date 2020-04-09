package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	"github.com/gobuffalo/envy"

	"github.com/Azure/azure-sdk-for-go/services/containerinstance/mgmt/2018-10-01/containerinstance"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-12-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"

	uuid "github.com/kthomas/go.uuid"
)

var (
	authorizer     autorest.Authorizer
	subscriptionID string
)

func main() {

	if err := envy.Load(); err != nil {
		println(fmt.Sprintf("env load error: %v", err.Error()))
		return
	}

	subscriptionID, err := envy.MustGet("SUBSCRIPTION_ID")
	if err != nil {
		print(fmt.Sprintf("expected env vars not provided: %+v", err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Second)
	defer cancel()

	groupName := "skynet"

	_, err = UpsertResourceGroup(ctx, groupName, subscriptionID)
	if err != nil {
		println(fmt.Sprintf("cannot create group: %v", err.Error()))
	}

	vnet, err := UpsertVirtualNetwork(ctx, subscriptionID, groupName, "skynet-vpc", "eastus")
	if err != nil {
		panic(fmt.Sprintf("virtual network creation failed"))
	}

	println(fmt.Sprintf("vnet: %+v", vnet))

	image := to.StringPtr("nginx:latest")

	container, err := StartContainer(ctx, "resGroup", image, to.StringPtr(groupName),
	subscriptionID, to.Float64Ptr(2), to.Float64Ptr(4), []string{}, []string, []string{"subnet1", "subnet2"}, 
	map[string]interface{}{}, "eastus", map[string]interface{}{})

	println(fmt.Sprintf("container: %+v", container))

	return
}

// StartContainer starts a new node in network
func StartContainer(ctx context.Context, resourceGroupName string,
	image *string, virtualNetworkID *string, subscriptionID string,
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

	securityGroups := make([]*string, 0)
	for i := range security {
		securityGroups = append(securityGroups, to.StringPtr(security[i]))
	}

	containerGroupName, _ := uuid.NewV4().String()
	cgClient, err := NewContainerGroupsClient(subscriptionID)
    future, err := cgClient.CreateOrUpdate(
		ctx,
		resourceGroupName,
		containerGroupName,
		containerinstance.ContainerGroup{
			Name: &containerGroupName,
			Location: &region,
			ContainerGroupProperties: &containerinstance.ContainerGroupProperties{
				IPAddress: &containerinstance.IPAddress{
					Type: containerinstance.Public,
					Ports: &[]containerinstance.Port{
						{
							Port:     to.Int32Ptr(80),
							Protocol: containerinstance.TCP,
						},
					},
				},
				OsType: containerinstance.Linux,
				Containers: &[]containerinstance.Container{
					{
						Name: to.StringPtr("gosdk-container"),
						ContainerProperties: &containerinstance.ContainerProperties{
							Ports: &[]containerinstance.ContainerPort{
								{
									Port: to.Int32Ptr(80),
								},
							},
							Image: to.StringPtr("nginx:latest"),
							Resources: &containerinstance.ResourceRequirements{
								Limits: &containerinstance.ResourceLimits{
									MemoryInGB: memory,
									CPU:        cpu,
								},
								Requests: &containerinstance.ResourceRequests{
									MemoryInGB: memory,
									CPU:        cpu,
								},
							},
						},
					},
				},
			},
		},
	)

	if err != nil {
		log.Fatalf("cannot create container group: %v", err)
	}

	err = future.WaitForCompletionRef(ctx, cgClient.Client)
	if err != nil {
		log.Fatalf("cannot create container group: %v", err)
	}
	return future.Result(cgClient)

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

// UpsertResourceGroup upserts a resource group for the given params
func UpsertResourceGroup(ctx context.Context, name, region string) (resources.Group, error) {
	gClient, err := NewResourceGroupsClient(region)
	group := resources.Group{
		Location: to.StringPtr("eastus"),
	}
	if err == nil {
		group, err := gClient.CreateOrUpdate(ctx, name, group)
		return group, err
	}
	// if err != nil {
	// 	log.Warningf("failed to upsert resource group: %s; %s", name, err.Error())
	// }
	return group, err
}

// UpsertVirtualNetwork upserts a resource group for the given params
func UpsertVirtualNetwork(ctx context.Context, subscriptionID, groupName, name, region string) (vnet network.VirtualNetwork, err error) {
	if err != nil {
		println("failed to upsert virtual network: %s; %s", name, err.Error())
		return vnet, err
	}
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
		return vnet, fmt.Errorf("cannot create virtual network: %v", err)
	}

	err = future.WaitForCompletionRef(ctx, vnetClient.Client)
	if err != nil {
		return vnet, fmt.Errorf("cannot get the vnet create or update future response: %v", err)
	}

	return future.Result(vnetClient)
}
