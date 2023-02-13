package functional

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"cloud.google.com/go/compute/apiv1/computepb"
	croGCP "github.com/integr8ly/cloud-resource-operator/pkg/providers/gcp"
	"github.com/integr8ly/cloud-resource-operator/pkg/providers/gcp/gcpiface"
	croResources "github.com/integr8ly/cloud-resource-operator/pkg/resources"
	"github.com/integr8ly/integreatly-operator/test/common"
	"google.golang.org/api/option"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	defaultIpRangePostfix      = "ip-range"
	defaultGcpIdentifierLength = 40
	gcpTier                    = "production"
	gcpAllowedCidrRanges       = []string{
		"10.255.255.255/8",
		"172.31.255.255/12",
	}
)

// TestGCPVPCExists tests GCP cloud network components
func TestGCPVPCExists(t common.TestingTB, testingCtx *common.TestingContext) {
	ctx := context.Background()

	// get the strategy map to get the GCP Subnet cidr block
	strategyMap := &corev1.ConfigMap{}
	err := testingCtx.Client.Get(ctx, types.NamespacedName{
		Namespace: common.RHOAMOperatorNamespace,
		Name:      croGCP.DefaultConfigMapName,
	}, strategyMap)
	if err != nil {
		t.Fatal("could not get gcp strategy map", err)
	}

	strat, err := getStrategyForResource(strategyMap, networkResourceType, gcpTier)
	if err != nil {
		t.Skip("_network key does not exist in strategy configmap, skipping standalone vpc network test")
	}

	// get the cidr block from Strategy Map
	expectedCidr, err := verifyAndGetCidrBlockFromGCPStrategyMap(strat)
	if err != nil {
		t.Fatal(err)
	}

	serviceAccountJson, err := getGCPCredentials(ctx, testingCtx.Client)
	if err != nil {
		t.Fatal("failed to retrieve gcp credentials %v", err)
	}

	projectID, err := croResources.GetGCPProject(ctx, testingCtx.Client)
	if err != nil {
		t.Fatal("error get Default Project ID %w", err)
	}

	_, err = verifyAddressRange(ctx, testingCtx.Client, projectID, expectedCidr, option.WithCredentialsJSON(serviceAccountJson))
	if err != nil {
		t.Fatal("error verifying gcp address range %w", err)
	}

	// verify service connection
	// verify peering

	// err = verifyClusterVpcAndSubnets(ctx, testingCtx.Client, projectID, strategyMapCidrBlock, option.WithCredentialsJSON(serviceAccountJson))
	// if err != nil {
	// 	t.Fatal("failed to get Vpc %w", err)
	// }

	// TODO
	clusterNodes := &corev1.NodeList{}
	err = testingCtx.Client.List(ctx, clusterNodes)
	if err != nil {
		t.Errorf("error when getting the list of OpenShift cluster nodes: %s", err)
	}
}

func verifyAndGetCidrBlockFromGCPStrategyMap(strat *strategyMap) (string, error) {
	vpcCreateConfig := &croGCP.CreateVpcInput{}
	if err := json.Unmarshal(strat.CreateStrategy, vpcCreateConfig); err != nil {
		return "", fmt.Errorf("failed to unmarshal vpc create config")
	}
	if vpcCreateConfig.CidrBlock != "" {
		if err := verifyCidrBlockIsInAllowedRange(vpcCreateConfig.CidrBlock, gcpAllowedCidrRanges); err != nil {
			return "", fmt.Errorf("cidr block %s is not within the allowed range %s", vpcCreateConfig.CidrBlock, err)
		}
	} else {
		fmt.Printf("strategy map CIDR block is empty")
	}
	return vpcCreateConfig.CidrBlock, nil
}

func verifyAddressRange(ctx context.Context, client k8sclient.Client, projectID string, expectedCidr string, opt option.ClientOption) (*computepb.Address, error) {
	addressClient, err := gcpiface.NewAddressAPI(ctx, opt)
	if err != nil {
		return nil, fmt.Errorf("error creating address client %w", err)
	}
	ipAddressName, err := croResources.BuildInfraName(ctx, client, defaultIpRangePostfix, defaultGcpIdentifierLength)
	if err != nil {
		return nil, fmt.Errorf("failed to create ip address range name %w", err)
	}

	address, err := addressClient.Get(ctx, &computepb.GetGlobalAddressRequest{
		Project: projectID,
		Address: ipAddressName,
	})
	if err != nil {
		return nil, fmt.Errorf("error retrieving address range %w", err)
	}
	if address.GetStatus() != computepb.Address_RESERVED.String() {
		return nil, fmt.Errorf("address range status expected RESERVED, but found %s", address.GetStatus())
	}
	if expectedCidr != "" {
		if cidr, err := strconv.Atoi(expectedCidr); err != nil && address.GetPrefixLength() != int32(cidr) {
			return nil, fmt.Errorf("address range cidr %d, does not match expected %s", address.GetPrefixLength(), expectedCidr)
		}
	}
	return address, nil
}

// func getClusterVpc(ctx context.Context, client k8sclient.Client, projectID string, opt option.ClientOption) (*computepb.Network, error) {
// 	clusterID, err := getClusterID(ctx, client)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to retrieve cluster id %w", err)
// 	}
// 	networkClient, err := gcpiface.NewNetworksAPI(ctx, opt)
// 	if err != nil {
// 		return nil, err
// 	}
// 	// get networks with a name that matches clusterID
// 	networks, err := networkClient.List(ctx, &computepb.ListNetworksRequest{
// 		Project: projectID,
// 		Filter:  utils.String(fmt.Sprintf("name = \"%s-*\"", clusterID)),
// 	})
// 	if err != nil {
// 		return nil, fmt.Errorf("error getting networks from gcp %w", err)
// 	}
// 	// confirm only one network matched the clusterID
// 	if len(networks) != 1 {
// 		return nil, fmt.Errorf("cannot determine cluster vpc. matching networks found %d", len(networks))
// 	}
// 	network := networks[0]

// 	// check the network has at least two subnets
// 	if len(network.GetSubnetworks()) < defaultNumberOfExpectedSubnets {
// 		return nil, fmt.Errorf("found cluster vpc has only %d subnetworks, expected at least 2", len(network.Subnetworks))
// 	}
// 	return network, nil
// }

// func getClusterSubnets(ctx context.Context, client k8sclient.Client, projectID string, clusterVpc *computepb.Network, opt option.ClientOption) ([]*computepb.Subnetwork, error) {
// 	subnetClient, err := gcpiface.NewSubnetsAPI(ctx, opt)
// 	if err != nil {
// 		return nil, err
// 	}
// 	var subnets []*computepb.Subnetwork
// 	clusterSubnets := clusterVpc.GetSubnetworks()
// 	for i := range clusterSubnets {
// 		name, region, err := parseSubnetUrl(clusterSubnets[i])
// 		if err != nil {
// 			return nil, err
// 		}
// 		subnet, err := subnetClient.Get(ctx, &computepb.GetSubnetworkRequest{
// 			Project:    projectID,
// 			Subnetwork: name,
// 			Region:     region,
// 		})
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to retrieve cluster subnet %s, %w", subnet, err)
// 		}
// 		subnets = append(subnets, subnet)
// 	}
// 	return subnets, nil
// }

// func verifyClusterVpcAndSubnets(ctx context.Context, c k8sclient.Client,
// 	projectID string, strategyMapCidrBlock string, clientOption option.ClientOption) error {

// 	clusterID, err := getClusterID(ctx, c)
// 	if err != nil {
// 		return errorUtil.Wrap(err, "error getting clusterID")
// 	}
// 	networkClient, err := gcpiface.NewNetworksAPI(ctx, clientOption)
// 	if err != nil {
// 		return errorUtil.Wrap(err, "failed to get NewNetworksAPI")
// 	}

// 	// get networks with a name that matches clusterID
// 	networks, err := networkClient.List(ctx, &computepb.ListNetworksRequest{
// 		Project: projectID,
// 		Filter:  utils.String(fmt.Sprintf("name = \"%s-*\"", clusterID)),
// 	})
// 	if err != nil {
// 		return errorUtil.Wrap(err, "error getting networks from gcp")
// 	}
// 	// confirm only one network matched the clusterID
// 	if len(networks) != 1 {
// 		return fmt.Errorf("cannot determine cluster vpc. matching networks found %d", len(networks))
// 	}
// 	clusterVpc := networks[0]

// 	subnets, err := getClusterSubnets(ctx, clusterVpc, projectID, clientOption)
// 	if err != nil {
// 		return fmt.Errorf("failed to get cluster subnetworks")
// 	}

// 	if len(subnets) < defaultNumberOfExpectedSubnets {
// 		return fmt.Errorf("found cluster vpc has only %d subnetworks, expected at least 2", len(clusterVpc.Subnetworks))
// 	}

// 	_, strategyMapCidrRange, err := net.ParseCIDR(strings.TrimSpace(strategyMapCidrBlock))
// 	if err != nil {
// 		return fmt.Errorf("cidr ip range validation failure, %w", err)
// 	}
// 	// validate that cidr range in strategy map is lower than or equal to /22
// 	if !isValidCIDRRange(strategyMapCidrRange) {
// 		return fmt.Errorf("%s is out of range, block sizes must be `/22` or lower, please update `_network` strategy", strategyMapCidrRange.String())
// 	}
// 	// validate Subnets cidr Range, and validate subnet cidr overlapping with Strategy map cidr
// 	err = validateSubnetsCidrRangeAndOverlapWithStartegyMapCidr(strategyMapCidrRange, subnets)
// 	if err != nil {
// 		return fmt.Errorf("cidr ip range validation failure, %w", err)
// 	}

// 	fmt.Printf("Cluster Vpc and Subnets - verified. Cluster %s , Vpc %s", clusterID, *clusterVpc.Name)
// 	return nil
// }

// func verifySubnets()

// func getClusterSubnets(ctx context.Context, clusterVpc *computepb.Network, projectID string, clientOption option.ClientOption) ([]*computepb.Subnetwork, error) {
// 	var subnets []*computepb.Subnetwork
// 	clusterSubnets := clusterVpc.GetSubnetworks()
// 	subnetsApi, err := gcpiface.NewSubnetsAPI(ctx, clientOption)
// 	if err != nil {
// 		return nil, err
// 	}
// 	for i := range clusterSubnets {
// 		name, region, err := parseSubnetUrl(clusterSubnets[i])
// 		if err != nil {
// 			return nil, err
// 		}
// 		subnet, err := subnetsApi.Get(ctx, &computepb.GetSubnetworkRequest{
// 			Project:    projectID,
// 			Subnetwork: name,
// 			Region:     region,
// 		})
// 		if err != nil {
// 			return nil, errorUtil.Wrapf(err, "failed to retrieve cluster subnet %s", subnet)
// 		}
// 		subnets = append(subnets, subnet)
// 	}
// 	return subnets, nil
// }
