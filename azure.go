package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	"github.com/gobuffalo/envy"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-12-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
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

	_, err = UpsertResourceGroup(ctx, "skynet", subscriptionID)
	if err != nil {
		println(fmt.Sprintf("cannot create group: %v", err.Error()))
	}

	vnet, err := UpsertVirtualNetwork(ctx, subscriptionID, "skynet", "skynet-vpc", "eastus")
	if err != nil {
		panic(fmt.Sprintf("virtual network creation failed"))
	}

	println(fmt.Sprintf("vnet: %+v", vnet))

	return
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
