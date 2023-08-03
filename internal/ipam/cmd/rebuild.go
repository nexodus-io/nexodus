package cmd

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/nexodus-io/nexodus/internal/ipam"
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/nexodus-io/nexodus/internal/util"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var defaultIPAMNamespace = uuid.UUID{}

func Rebuild(ctx context.Context, log *zap.Logger, db *gorm.DB, ipam ipam.IPAM) error {

	type result struct {
		ID             uuid.UUID      `gorm:"type:uuid;primary_key;" json:"id" example:"aa22666c-0f57-45cb-a449-16efecc04f2e"`
		TunnelIP       string         `json:"tunnel_ip"`
		TunnelIpV6     string         `json:"tunnel_ip_v6"`
		ChildPrefix    pq.StringArray `json:"child_prefix" gorm:"type:text[]" swaggertype:"array,string"`
		OrganizationID uuid.UUID      `json:"organization_id"`
		PrivateCidr    bool           `json:"private_cidr"`
		IpCidr         string         `json:"ip_cidr"`
		IpCidrV6       string         `json:"ip_cidr_v6"`
	}
	rows, err := db.Model(&models.Device{}).
		Select("devices.id, organization_id, tunnel_ip, tunnel_ip_v6, child_prefix, private_cidr, ip_cidr, ip_cidr_v6").
		Joins("inner join organizations on organizations.id::text = devices.organization_id").
		Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var device result
		err = db.ScanRows(rows, &device)
		if err != nil {
			return err
		}

		log.Info("processing", zap.String("device", device.ID.String()))

		ipamNamespace := defaultIPAMNamespace
		if device.PrivateCidr {
			ipamNamespace = device.OrganizationID
		}

		if err := ipam.CreateNamespace(ctx, ipamNamespace); err != nil {
			return fmt.Errorf("failed to create ipam namespace: %w", err)
		}

		if err := ipam.AssignPrefix(ctx, ipamNamespace, device.IpCidr); err != nil {
			return fmt.Errorf("can't assign default ipam v4 prefix: %w", err)
		}
		if err := ipam.AssignPrefix(ctx, ipamNamespace, device.IpCidrV6); err != nil {
			return fmt.Errorf("can't assign default ipam v6 prefix: %w", err)
		}

		if device.TunnelIP != "" {
			err = ipam.AcquireIP(ctx, ipamNamespace, device.IpCidr, device.TunnelIP)
			if err != nil {
				log.Sugar().Warnf("Failed to allocate ip %s for device %s", device.TunnelIP, device.ID)
			}
		}

		if device.TunnelIpV6 != "" {
			err = ipam.AcquireIP(ctx, ipamNamespace, device.IpCidrV6, device.TunnelIpV6)
			if err != nil {
				log.Sugar().Warnf("Failed to allocate ip %s for device %s", device.TunnelIpV6, device.ID)
			}
		}

		// allocate a child prefix if requested
		for _, prefix := range device.ChildPrefix {
			if !util.IsValidPrefix(prefix) {
				return fmt.Errorf("invalid cidr detected in the child prefix field of %s", prefix)
			}
			// Skip the prefix assignment if it's an IPv4 or IPv6 default route
			if !util.IsDefaultIPv4Route(prefix) && !util.IsDefaultIPv6Route(prefix) {
				if err := ipam.AssignPrefix(ctx, ipamNamespace, prefix); err != nil {
					return fmt.Errorf("failed to assign child prefix: %w", err)
				}
			}
		}

	}
	return nil
}
