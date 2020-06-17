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

var tc = &provide.TargetCredentials{}

func TestListNodes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Second)
	defer cancel()

	resourceGroupName := "blockchain"

	list, err := ListTransactionNodes(ctx, tc, "hellomember", resourceGroupName)
	if err != nil {
		println(fmt.Sprintf("cannot list nodes: %v", err.Error()))
	}

	node := list[0]
	println(fmt.Sprintf("node: %+v", node))
	println(fmt.Sprintf("props: %+v", node.TransactionNodeProperties))
	println(fmt.Sprintf("key: %s", *node.TransactionNodeProperties.PublicKey))
	println(fmt.Sprintf("dns: %s", *node.TransactionNodeProperties.DNS))
	// println(fmt.Sprintf("list of nodes: %+v", list))
}

func TestTransNode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Second)
	defer cancel()

	resourceGroupName := "blockchain"

	future, err := CreateTransactionNode(ctx, tc, "hellomember", "hellonode3", resourceGroupName)
	if err != nil {
		println(fmt.Sprintf("cannot create node: %v", err.Error()))
	}

	log.Warningf("blockchain node create complete")
	println(fmt.Sprintf("future node: %+v", *future))
}

func TestMemberClient(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Second)
	defer cancel()

	resourceGroupName := "blockchain"

	future, err := CreateBlockchainMemberFuture(ctx, tc, resourceGroupName, "hellomember")
	if err != nil {
		println(fmt.Sprintf("cannot create member: %v", err.Error()))
	}

	member, err := CreateBlockchainMemberResult(ctx, tc, future)
	if err != nil {
		log.Warningf("failed to get the member creation complete; %s", err.Error())
	}

	log.Warningf("blockchain member create complete")
	println(fmt.Sprintf("member: %+v", *member))
}

func TestListMembers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Second)
	defer cancel()

	resourceGroupName := "prvd-abs"

	memberList, err := GetBlockchainMembersList(ctx, tc, resourceGroupName)
	if err != nil {
		println(fmt.Sprintf("cannot list members: %v", err.Error()))
	}
	member := memberList.Values()[0]
	len := len(memberList.Values())
	println(fmt.Sprintf("member length: %d", len))
	println(fmt.Sprintf("member location: %s", *member.Location))
	println(fmt.Sprintf("member: %+v", *member.MemberProperties.UserName))
}

// func TestTransactionNode(t *testing.T) {
// 	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Second)
// 	defer cancel()

// 	blockchainMemberName := "provide"
// 	transactionNodeName := "node1"
// 	resourceGroupName := "prvd-abs"

// 	node, err := CreateTransactionNode(ctx, tc, blockchainMemberName, transactionNodeName, resourceGroupName)
// 	if err != nil {
// 		println(fmt.Sprintf("cannot create node: %v", err.Error()))
// 	}
// 	println(fmt.Sprintf("node: %+v", *node))
// }

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

	// containerGroupName, _ := uuid.NewV4()
	cgroupName := "a12c192c-2fba-4696-9418-845439c3e47b"
	name := "nats-server"

	params := &provide.ContainerParams{
		Region:             region,
		ResourceGroupName:  groupName,
		Image:              image,
		ContainerGroupName: to.StringPtr(cgroupName),
		ContainerName:      to.StringPtr(name),
		VirtualNetworkID:   to.StringPtr(vnetName),
		CPU:                to.Int64Ptr(2),
		Memory:             to.Int64Ptr(4),
		Entrypoint:         []*string{},
		SecurityGroupIds:   []string{},
		SubnetIds:          []string{"subnet1", "subnet2"},
		Environment:        map[string]interface{}{},
		Security:           security,
	}

	_, err := UpsertResourceGroup(ctx, tc, region, groupName)
	if err != nil {
		println(fmt.Sprintf("cannot create group: %v", err.Error()))
	}

	// container, ids, err := StartContainer(params)
	result, err := StartContainer(params, tc)
	if err != nil {
		panic(fmt.Sprintf("%s", err.Error()))
	}
	println(fmt.Sprintf("container ids: %+v", result.ContainerIds))
	println(fmt.Sprintf("container network: %+v", result.ContainerInterfaces[0]))
	println(fmt.Sprintf("container ip: %s", *result.ContainerInterfaces[0].IPv4))
	id := result.ContainerIds[0]
	println(fmt.Sprintf("container id: %s", id))

	logs, err := ContainerLogs(context.TODO(), tc, groupName, id, id, nil)
	if err != nil {
		panic(fmt.Sprintf("%s", err.Error()))
	}
	println(fmt.Sprintf("container ids: %+v", logs.Content))
	println(fmt.Sprintf("container ids: %+v", logs))

	// result, err = StartContainer(params, tc)
	// if err != nil {
	// 	panic(fmt.Sprintf("%s", err.Error()))
	// }
	// println(fmt.Sprintf("container: %+v", result.ContainerIds))
	// println(fmt.Sprintf("container network: %+v", result.ContainerInterfaces[0]))
	// id = result.ContainerIds[0]
	// println(fmt.Sprintf("container id: %s", id))

	// id := "af0cca54-5883-4394-b876-db9839e76084"
	// DeleteContainer(ctx, subscriptionID, groupName, id)

}

func TestLogs(t *testing.T) {
	id := "a12c192c-2fba-4696-9418-845439c3e47b"
	id2 := "nats-server"
	groupName := "skynet"

	logs, err := ContainerLogs(context.TODO(), tc, groupName, id, id2, nil)
	if err != nil {
		panic(fmt.Sprintf("%s", err.Error()))
	}
	println(fmt.Sprintf("log content: %+v", logs.Content))
	var b []byte
	logs.Response.Body.Read(b)
	println(fmt.Sprintf("response content: %+v", logs.Response.Response))
	fmt.Println(b)

	println(fmt.Sprintf("logs: %+v", logs))
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
