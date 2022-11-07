package ipam

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/bufbuild/connect-go"
	apiv1 "github.com/metal-stack/go-ipam/api/v1"
	"github.com/metal-stack/go-ipam/api/v1/apiv1connect"
	log "github.com/sirupsen/logrus"
)

type IPAM struct {
	client apiv1connect.IpamServiceClient
}

func NewIPAM(ipamAddress string) IPAM {
	return IPAM{
		client: apiv1connect.NewIpamServiceClient(
			http.DefaultClient,
			ipamAddress,
			connect.WithGRPC(),
		)}
}

func (i *IPAM) AssignSpecificNodeAddress(ctx context.Context, ipamPrefix string, nodeAddress string) (string, error) {
	if err := validateIP(nodeAddress); err != nil {
		return "", fmt.Errorf("Address %s is not valid", nodeAddress)
	}
	res, err := i.client.AcquireIP(ctx, connect.NewRequest(&apiv1.AcquireIPRequest{
		PrefixCidr: ipamPrefix,
		Ip:         &nodeAddress,
	}))
	if err != nil {
		log.Errorf("failed to assign the requested address %s, assigning an address from the pool: %v\n", nodeAddress, err)
		return i.AssignFromPool(ctx, ipamPrefix)
	}
	return res.Msg.Ip.Ip, nil
}

func (i *IPAM) AssignFromPool(ctx context.Context, ipamPrefix string) (string, error) {
	res, err := i.client.AcquireIP(ctx, connect.NewRequest(&apiv1.AcquireIPRequest{
		PrefixCidr: ipamPrefix,
	}))
	if err != nil {
		log.Errorf("failed to acquire an IPAM assigned address %v", err)
		return "", fmt.Errorf("failed to acquire an IPAM assigned address %v\n", err)
	}
	return res.Msg.Ip.Ip, nil
}

func (i *IPAM) AssignPrefix(ctx context.Context, cidr string) error {
	cidr, err := cleanCidr(cidr)
	if err != nil {
		log.Errorf("invalid prefix requested: %v", err)
		return fmt.Errorf("invalid prefix requested: %v", err)
	}
	_, err = i.client.CreatePrefix(ctx, connect.NewRequest(&apiv1.CreatePrefixRequest{Cidr: cidr}))
	return err
}

// cleanCidr ensures a valid IP4/IP6 address is provided and return a proper
// network prefix if the network address if the network address was not precise.
// example: if a user provides 192.168.1.1/24 we will infer 192.168.1.0/24.
func cleanCidr(uncheckedCidr string) (string, error) {
	_, validCidr, err := net.ParseCIDR(uncheckedCidr)
	if err != nil {
		return "", err
	}
	return validCidr.String(), nil
}

// ValidateIP ensures a valid IP4/IP6 address is provided
func validateIP(ip string) error {
	if ip := net.ParseIP(ip); ip != nil {
		return nil
	}
	return fmt.Errorf("%s is not a valid v4 or v6 IP", ip)
}
