package controltower

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/redhat-et/jaywalking/internal/controltower/ipam"
	log "github.com/sirupsen/logrus"
)

type Zone struct {
	ID          uuid.UUID
	Peers       map[uuid.UUID]struct{}
	Name        string
	Description string
	IpCidr      string
	ZoneIpam    ipam.AirliftIpam
	HubZone     bool
}

func NewZone(id uuid.UUID, name, description, cidr string, hubZone bool) (*Zone, error) {
	zoneIpamSaveFile := fmt.Sprintf("%s.json", id.String())
	// TODO: until we save control tower state between restarts, the ipam save file will be out of sync
	// new zones will delete the stale IPAM file on creation.
	// currently this will delete and overwrite an existing zone and ipam objects.
	if fileExists(zoneIpamSaveFile) {
		log.Warnf("ipam persistent storage file [ %s ] already exists on the control tower, deleting it", zoneIpamSaveFile)
		if err := deleteFile(zoneIpamSaveFile); err != nil {
			return nil, fmt.Errorf("unable to delete the ipam persistent storage file on the control tower [ %s ]: %v", zoneIpamSaveFile, err)
		}
	}
	ipam, err := ipam.NewIPAM(context.Background(), zoneIpamSaveFile, cidr)
	if err != nil {
		return nil, err
	}
	if err := ipam.IpamSave(context.Background()); err != nil {
		log.Errorf("failed to save the ipam persistent storage file %v", err)
		return nil, err
	}
	return &Zone{
		ID:          id,
		Peers:       make(map[uuid.UUID]struct{}),
		Name:        name,
		Description: description,
		IpCidr:      cidr,
		ZoneIpam:    *ipam,
		HubZone:     hubZone,
	}, nil
}

func (z *Zone) MarshalJSON() ([]byte, error) {
	peers := make([]uuid.UUID, 0)
	for k := range z.Peers {
		peers = append(peers, k)
	}
	return json.Marshal(
		struct {
			ID          uuid.UUID   `json:"id"`
			Peers       []uuid.UUID `json:"peers"`
			Name        string      `json:"name"`
			Description string      `json:"description"`
			IpCidr      string      `json:"cidr"`
			HubZone     bool        `json:"hub-zone"`
		}{
			ID:          z.ID,
			Peers:       peers,
			Name:        z.Name,
			Description: z.Description,
			IpCidr:      z.IpCidr,
			HubZone:     z.HubZone,
		})
}

func fileExists(f string) bool {
	if _, err := os.Stat(f); err != nil {
		return false
	}
	return true
}

func deleteFile(f string) error {
	if err := os.Remove(f); err != nil {
		return err
	}
	return nil
}
