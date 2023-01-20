package ipam

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/bufbuild/connect-go"
	apiv1 "github.com/metal-stack/go-ipam/api/v1"
	"github.com/metal-stack/go-ipam/api/v1/apiv1connect"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer trace.Tracer

func init() {
	tracer = otel.Tracer("github.com/redhat-et/apex/internal/ipam")
}

type IPAM struct {
	logger *zap.SugaredLogger
	client apiv1connect.IpamServiceClient
}

func NewIPAM(logger *zap.SugaredLogger, ipamAddress string) IPAM {
	return IPAM{
		logger: logger,
		client: apiv1connect.NewIpamServiceClient(
			http.DefaultClient,
			ipamAddress,
			connect.WithGRPC(),
		)}
}

func (i *IPAM) AssignSpecificTunnelIP(parent context.Context, ipamPrefix string, TunnelIP string) (string, error) {
	ctx, span := tracer.Start(parent, "AssignSpecificTunnelIP")
	defer span.End()
	if err := validateIP(TunnelIP); err != nil {
		return "", fmt.Errorf("Address %s is not valid", TunnelIP)
	}
	res, err := i.client.AcquireIP(ctx, connect.NewRequest(&apiv1.AcquireIPRequest{
		PrefixCidr: ipamPrefix,
		Ip:         &TunnelIP,
	}))
	if err != nil {
		i.logger.Errorf("failed to assign the requested address %s, assigning an address from the pool: %v\n", TunnelIP, err)
		return i.AssignFromPool(ctx, ipamPrefix)
	}
	return res.Msg.Ip.Ip, nil
}

func (i *IPAM) AssignFromPool(parent context.Context, ipamPrefix string) (string, error) {
	ctx, span := tracer.Start(parent, "AssignFromPool")
	defer span.End()
	res, err := i.client.AcquireIP(ctx, connect.NewRequest(&apiv1.AcquireIPRequest{
		PrefixCidr: ipamPrefix,
	}))
	if err != nil {
		return "", fmt.Errorf("failed to acquire an IPAM assigned address %w\n", err)
	}
	return res.Msg.Ip.Ip, nil
}

func (i *IPAM) AssignPrefix(parent context.Context, cidr string) error {
	ctx, span := tracer.Start(parent, "AssignPrefix")
	defer span.End()
	cidr, err := cleanCidr(cidr)
	if err != nil {
		return fmt.Errorf("invalid prefix requested: %w", err)
	}

	_, originalErr := i.client.CreatePrefix(ctx, connect.NewRequest(&apiv1.CreatePrefixRequest{Cidr: cidr}))
	if originalErr != nil {
		// check to see if the prefix had been already created....
		resp, err := i.client.GetPrefix(ctx, connect.NewRequest(&apiv1.GetPrefixRequest{Cidr: cidr}))
		if err == nil {
			// it did exist... so ignore that create error since the prefix was created.
			if resp.Msg.Prefix.Cidr == cidr && resp.Msg.Prefix.ParentCidr == "" {
				originalErr = nil
			}
		}
	}
	return originalErr
}

// ReleaseToPool release the ipam address back to the specified prefix
func (i *IPAM) ReleaseToPool(ctx context.Context, address, cidr string) error {
	_, err := i.client.ReleaseIP(ctx, connect.NewRequest(&apiv1.ReleaseIPRequest{
		Ip:         address,
		PrefixCidr: cidr,
	}))

	if err != nil {
		return fmt.Errorf("failed to release IPAM address %w", err)
	}
	return nil
}

// ReleasePrefix release the ipam address back to the specified prefix
func (i *IPAM) ReleasePrefix(ctx context.Context, cidr string) error {
	_, err := i.client.DeletePrefix(ctx, connect.NewRequest(&apiv1.DeletePrefixRequest{
		Cidr: cidr,
	}))

	if err != nil {
		return fmt.Errorf("failed to release IPAM prefix %w", err)
	}
	return nil
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
