package nexodus

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"syscall"

	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/client"
	"github.com/nexodus-io/nexodus/internal/models"
	"go.uber.org/zap"
	"golang.org/x/term"
)

type ApiServer struct {
	username      string
	password      string
	controllerURL *url.URL
	ControllerIP  string
	client        *client.Client
	logger        *zap.SugaredLogger
	skipTlsVerify bool
}

func NewApiServer(username string,
	password string,
	controller string,
	logger *zap.SugaredLogger,
	skipTlsVerify bool,
) (*ApiServer, error) {

	controllerURL, err := url.Parse(controller)
	if err != nil {
		return nil, err
	}

	// Force controller URL be api.${DOMAIN}
	controllerURL.Host = "api." + controllerURL.Host
	controllerURL.Path = ""

	as := &ApiServer{
		username:      username,
		password:      password,
		ControllerIP:  controller,
		controllerURL: controllerURL,
		logger:        logger,
		skipTlsVerify: skipTlsVerify,
	}

	return as, nil
}

func (as *ApiServer) Connect(ctx context.Context, fn func(string)) error {
	var options []client.Option
	if as.username == "" {
		options = append(options, client.WithDeviceFlow())
	} else if as.username != "" && as.password == "" {
		fmt.Print("Enter nexodus account password: ")
		passwdInput, err := term.ReadPassword(int(syscall.Stdin))
		println()
		if err != nil {
			return fmt.Errorf("login aborted: %w", err)
		}
		as.password = string(passwdInput)
		options = append(options, client.WithPasswordGrant(as.username, as.password))
	} else {
		options = append(options, client.WithPasswordGrant(as.username, as.password))
	}
	if as.skipTlsVerify { // #nosec G402
		options = append(options, client.WithTLSConfig(&tls.Config{
			InsecureSkipVerify: true,
		}))
	}

	client, err := client.NewClient(ctx, as.controllerURL.String(), fn, options...)
	if err != nil {
		return err
	}

	as.client = client
	return nil
}

func (as *ApiServer) GetCurrentUser() (models.UserJSON, error) {
	return as.client.GetCurrentUser()
}

func (as *ApiServer) GetOrganizations() ([]models.OrganizationJSON, error) {
	return as.client.GetOrganizations()
}

// getPeerListing return the peer listing for the current user account
func (as *ApiServer) GetPeerListing(org uuid.UUID) ([]models.Device, error) {
	peerListing, err := as.client.GetDeviceInOrganization(org)
	if err != nil {
		return nil, err
	}

	return peerListing, nil
}

func (as *ApiServer) CreateDevice(device models.AddDevice) (models.Device, error) {
	return as.client.CreateDevice(device)
}

func (as *ApiServer) UpdateDevice(uuid uuid.UUID, device models.UpdateDevice) (models.Device, error) {
	return as.client.UpdateDevice(uuid, device)
}

// OrgRelayCheck checks if there is an existing Relay node in the organization that does not match this device's pub key
func OrgRelayCheck(peerListing []models.Device, wgPubKey string) (uuid.UUID, error) {
	var relayID uuid.UUID
	for _, p := range peerListing {
		if p.Relay && wgPubKey != p.PublicKey {
			return p.ID, nil
		}
	}

	return relayID, nil
}

// OrgDiscoveryCheck checks if there is an existing Discovery node in the organization that does not match this device's pub key
func OrgDiscoveryCheck(peerListing []models.Device, wgPubKey string) (uuid.UUID, error) {
	var discoveryID uuid.UUID
	for _, p := range peerListing {
		if p.Discovery && wgPubKey != p.PublicKey {
			return p.ID, nil
		}
	}

	return discoveryID, nil
}
