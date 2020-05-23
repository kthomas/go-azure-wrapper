package azurewrapper

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Azure/go-autorest/autorest/to"
	provide "github.com/provideservices/provide-go"
)

// type testData struct {
// 	subscriptionID string
// 	region         string
// 	groupName      string
// 	lbName         string
// 	ipName         string
// 	security       map[string]interface{}
// 	image          string
// }

var tc = &provide.TargetCredentials{
	AzureSubscriptionID: to.StringPtr(""),
	AzureClientID:       to.StringPtr(""),
	AzureClientSecret:   to.StringPtr(""),
	AzureTenantID:       to.StringPtr(""),
}

func TestStartContainer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Second)
	defer cancel()
	region := "eastus"
	groupName := "skynet"
	vnetName := "skynet-vnet"
	security := map[string]interface{}{
		"egress": "*",
		"ingress": map[string]interface{}{
			"0.0.0.0/0": map[string]interface{}{
				"tcp": []interface{}{
					float64(4221),
					float64(4222),
				},
				"udp": []interface{}{},
			},
		},
	}
	image := to.StringPtr("provide/nats-server:latest")

	params := &provide.ContainerParams{
		Region:            region,
		ResourceGroupName: groupName,
		Image:             image,
		VirtualNetworkID:  to.StringPtr(vnetName),
		CPU:               to.Int64Ptr(2),
		Memory:            to.Int64Ptr(4),
		Entrypoint:        []*string{},
		SecurityGroupIds:  []string{},
		SubnetIds:         []string{"subnet1", "subnet2"},
		Environment:       map[string]interface{}{},
		Security:          security,
	}

	_, err := UpsertResourceGroup(ctx, tc, region, groupName)
	if err != nil {
		println(fmt.Sprintf("cannot create group: %v", err.Error()))
	}

	// container, ids, err := StartContainer(params)
	result := StartContainer(params, tc)
	if result.Err != nil {
		panic(fmt.Sprintf("%s", result.Err.Error()))
	}
	println(fmt.Sprintf("container: %+v", result.ContainerIds))
	id := result.ContainerIds[0]
	println(fmt.Sprintf("container id: %s", id))

	result = StartContainer(params, tc)
	if result.Err != nil {
		panic(fmt.Sprintf("%s", result.Err.Error()))
	}
	println(fmt.Sprintf("container: %+v", result.ContainerIds))
	id = result.ContainerIds[0]
	println(fmt.Sprintf("container id: %s", id))

	// id := "af0cca54-5883-4394-b876-db9839e76084"
	// DeleteContainer(ctx, subscriptionID, groupName, id)

}

func TestUpsertResourceGroup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Second)
	defer cancel()
	region := "eastus"
	groupName := "skynetTest"
	_, err := UpsertResourceGroup(ctx, tc, region, groupName)
	if err != nil {
		println(fmt.Sprintf("cannot create group: %v", err.Error()))
	}

	res, err := DeleteResourceGroup(ctx, tc, groupName)
	if err != nil {
		panic(fmt.Sprintf("DeleteResourceGroup failed"))
	}
	println(fmt.Sprintf("DeleteResourceGroup res: %t", res))
}

func TestUpsertVirtualNetwork(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Second)
	defer cancel()
	region := "eastus"
	groupName := "skynet"
	vnetName := "skynet-vpc"
	vnet, err := UpsertVirtualNetwork(ctx, tc, groupName, vnetName, region)
	if err != nil {
		panic(fmt.Sprintf("virtual network creation failed"))
	}
	println(fmt.Sprintf("vnet: %+v", vnet))

	res, err := DeleteVirtuaNetwork(ctx, tc, groupName, vnetName)
	println(fmt.Sprintf("delete virtual network: %t", res))
}

func TestCreateLoadBalancer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Second)
	defer cancel()
	region := "eastus"
	groupName := "skynet"
	lbName := "balancer"
	ipName := "publicSkynetIP"
	security := map[string]interface{}{
		"egress": "*",
		"ingress": map[string]interface{}{
			"0.0.0.0/0": map[string]interface{}{
				"tcp": []interface{}{
					float64(4221),
					float64(4222),
				},
				"udp": []interface{}{},
			},
		},
	}
	ip, err := CreatePublicIP(ctx, ipName, region, groupName, tc)
	println(fmt.Sprintf("ip: %+v", ip))
	lb, err := CreateLoadBalancer(ctx, lbName, region, ipName, groupName, tc, security)
	if err != nil {
		println(fmt.Sprintf("cannot create load balancer: %v", err.Error()))
	}
	println(fmt.Sprintf("balancer: %+v", lb))

	res, err := DeleteLoadBalancer(ctx, lbName, groupName, tc)
	println(res)
	if err != nil {
		println(fmt.Sprintf("cannot delete load balancer: %v", err.Error()))
	}
}
