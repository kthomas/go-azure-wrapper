package azurewrapper

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Azure/go-autorest/autorest/to"
)

func TestStartContainer(t *testing.T) {
	subscriptionID := os.Getenv("SUBSCRIPTION_ID")
	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Second)
	defer cancel()
	region := "eastus"
	groupName := "skynet"
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

	params := &AzureContainerCreateParams{
		context:           ctx,
		subscriptionID:    subscriptionID,
		region:            region,
		resourceGroupName: groupName,
		image:             image,
		virtualNetworkID:  to.StringPtr(groupName),
		cpu:               to.Int64Ptr(2),
		memory:            to.Int64Ptr(4),
		entrypoint:        []*string{},
		securityGroupIds:  []string{},
		subnetIds:         []string{"subnet1", "subnet2"},
		environment:       map[string]interface{}{},
		security:          security,
	}

	// container, ids, err := StartContainer(params)
	result := StartContainer(params)
	if result.err != nil {
		panic(fmt.Sprintf("%s", result.err.Error()))
	}
	println(fmt.Sprintf("container: %+v", result.containerIds))
	id := result.ids[0]
	println(fmt.Sprintf("container id: %s", id))
	// id := "af0cca54-5883-4394-b876-db9839e76084"
	DeleteContainer(ctx, subscriptionID, groupName, id)
}
