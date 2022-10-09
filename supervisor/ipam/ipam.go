package ipam

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"

	goipam "github.com/metal-stack/go-ipam"
	log "github.com/sirupsen/logrus"
)

type SupIpam struct {
	PersistFile string
	ZoneIpam    goipam.Ipamer
	Prefix      *goipam.Prefix
}

func NewIPAM(ctx context.Context, saveFile, cidr string) (*SupIpam, error) {
	ipamer := goipam.New()

	prefix, err := ipamer.NewPrefix(ctx, cidr)
	if err != nil {
		return nil, err
	}

	ipam := &SupIpam{
		PersistFile: saveFile,
		ZoneIpam:    ipamer,
		Prefix:      prefix,
	}
	if err := ipam.loadData(ctx); err != nil {
		return nil, err
	}
	return ipam, nil
}

func (si *SupIpam) loadData(ctx context.Context) error {
	if _, err := os.Stat(si.PersistFile); os.IsNotExist(err) {
		return nil
	}
	err := si.IpamDeleteAllPrefixes(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete prefixes for loading %w", err)
	}

	b, err := ioutil.ReadFile(si.PersistFile)
	if err != nil {
		return err
	}
	return si.ZoneIpam.Load(ctx, string(b))
}

func (si *SupIpam) RequestSpecificIP(ctx context.Context, requestedIP, prefix string) (string, error) {
	err := si.ZoneIpam.ReleaseIPFromPrefix(ctx, prefix, requestedIP)
	if err != nil {
		log.Warnln("failed to release requested address from IPAM")
	}
	ip, err := si.ZoneIpam.AcquireSpecificIP(ctx, prefix, requestedIP)
	if err != nil {
		return "", fmt.Errorf("%v", err)
	}
	return ip.IP.String(), err
}

func (si *SupIpam) RequestIP(ctx context.Context, prefix string) (string, error) {
	ip, err := si.ZoneIpam.AcquireIP(ctx, prefix)
	if err != nil {
		return "", fmt.Errorf("%v", err)
	}
	return ip.IP.String(), err
}

func (si *SupIpam) RequestChildPrefix(ctx context.Context, prefix string) (string, error) {
	cidr, err := cleanCidr(prefix)
	if err != nil {
		return "", fmt.Errorf("invalid child prefix requested: %v", err)
	}
	childPrefix, err := si.ZoneIpam.NewPrefix(ctx, cidr)
	if err != nil {
		return "", fmt.Errorf("failed to allocate requested child prefix %v", err)
	}
	return childPrefix.Cidr, nil
}

func (si *SupIpam) IpamSave(ctx context.Context) error {
	if si.PersistFile == "" {
		return nil
	}
	data, err := si.ZoneIpam.Dump(ctx)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(si.PersistFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_SYNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(data)
	return err
}

func (si *SupIpam) IpamDeleteAllPrefixes(ctx context.Context) error {
	prefixes, err := si.ZoneIpam.ReadAllPrefixCidrs(ctx)
	if err != nil {
		return fmt.Errorf("failed to read prefixes for deletion %w", err)
	}
	for _, prefix := range prefixes {
		_, err := si.ZoneIpam.DeletePrefix(ctx, prefix)
		if err != nil {
			return err
		}
	}
	return nil
}

// validateIp ensures a valid IP4/IP6 address is provided and return a proper
// network prefix if the network address if the network address was not precise.
// example: if a user provides 192.168.1.1/24 we will infer 192.168.1.0/24.
func cleanCidr(uncheckedCidr string) (string, error) {
	_, validCidr, err := net.ParseCIDR(uncheckedCidr)
	if err != nil {
		return "", err
	}
	return validCidr.String(), nil
}

// ValidateIp ensures a valid IP4/IP6 address is provided
func ValidateIp(ip string) error {
	if ip := net.ParseIP(ip); ip != nil {
		return nil
	}
	return fmt.Errorf("%s is not a valid v4 or v6 IP", ip)
}
