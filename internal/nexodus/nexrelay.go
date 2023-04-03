package nexodus

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/client"
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/nexodus-io/nexodus/internal/nexodus/wireguard"
	"github.com/nexodus-io/nexodus/internal/util"
	"go.uber.org/zap"
)

type NexRelay struct {
	wg            *wireguard.WireGuard
	apiserver     *ApiServer
	nex           *Nexodus
	relay         bool
	discoveryNode bool
	logger        *zap.SugaredLogger
}

func NewNexRelay(ctx context.Context,
	logger *zap.SugaredLogger,
	controller string,
	username string,
	password string,
	wgListenPort int,
	wireguardPubKey string,
	wireguardPvtKey string,
	requestedIP string,
	userProvidedLocalIP string,
	stun bool,
	discoveryNode bool,
	insecureSkipTlsVerify bool,
	version string,
) (*NexRelay, error) {

	wg, err := wireguard.NewWireGuard(wireguardPubKey, wireguardPvtKey, wireguard.WgDefaultPort, true, logger, false)
	if err != nil {
		return nil, err
	}

	apiserver, err := NewApiServer(username, password, controller, logger, insecureSkipTlsVerify)
	if err != nil {
		return nil, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	nexodus := &Nexodus{
		requestedIP:         requestedIP,
		userProvidedLocalIP: userProvidedLocalIP,
		stun:                stun,
		deviceCache:         make(map[uuid.UUID]models.Device),
		hostname:            hostname,
		status:              NexdStatusStarting,
		version:             version,
		logger:              logger,
	}

	nexr := &NexRelay{
		wg:            wg,
		apiserver:     apiserver,
		nex:           nexodus,
		relay:         true,
		discoveryNode: discoveryNode,
		logger:        logger,
	}

	if err := nexr.checkUnsupportedConfigs(); err != nil {
		return nil, err
	}

	// remove orphaned wg interfaces from previous node joins
	nexr.wg.RemoveExistingInterface()

	reflexiveAddress, err := getReflexiveAddress(nexr.logger, nexr.wg.ListenPort)
	if err != nil {
		nexr.logger.Warn(err)
	}
	nexr.nex.nodeReflexiveAddress = reflexiveAddress

	return nexr, nil
}

func (nexr *NexRelay) SetStatus(status int, msg string) {
	nexr.nex.statusMsg = msg
	nexr.nex.status = status
}

func (nexr *NexRelay) Start(ctx context.Context, wg *sync.WaitGroup) error {
	var err error

	if err := nexr.nex.CtlServerStart(ctx, wg); err != nil {
		return fmt.Errorf("CtlServerStart(): %w", err)
	}

	err = nexr.apiserver.Connect(ctx, func(msg string) {
		nexr.SetStatus(NexdStatusAuth, msg)
	})
	if err != nil {
		return err
	}

	nexr.SetStatus(NexdStatusRunning, "")

	if err := nexr.wg.HandleKeys(); err != nil {
		return fmt.Errorf("handleKeys: %w", err)
	}

	user, err := nexr.apiserver.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("get user error: %w", err)
	}

	organizations, err := nexr.apiserver.GetOrganizations()
	if err != nil {
		return fmt.Errorf("get organizations error: %w", err)
	}

	if len(organizations) == 0 {
		return fmt.Errorf("user does not belong to any organizations")
	}
	if len(organizations) != 1 {
		return fmt.Errorf("user being in > 1 organization is not yet supported")
	}
	nexr.logger.Infof("Device belongs in organization: %s (%s)", organizations[0].Name, organizations[0].ID)
	nexr.nex.organization = organizations[0].ID

	var localIP string
	var localEndpointPort int

	// User requested ip --request-ip takes precedent
	if nexr.nex.userProvidedLocalIP != "" {
		localIP = nexr.nex.userProvidedLocalIP
		localEndpointPort = nexr.wg.ListenPort
	}

	peerListing, err := nexr.apiserver.GetPeerListing(nexr.nex.organization)
	if err != nil {
		return err
	}

	if nexr.relay {
		existingRelay, err := OrgRelayCheck(peerListing, nexr.wg.WireguardPubKey)
		if err != nil {
			return err
		}
		if existingRelay != uuid.Nil {
			return fmt.Errorf("the organization already contains a relay node, device %s needs to be deleted before adding a new relay", existingRelay)
		}
	}

	if nexr.discoveryNode {
		existingDiscoveryNode, err := OrgDiscoveryCheck(peerListing, nexr.wg.WireguardPubKey)
		if err != nil {
			return err
		}
		if existingDiscoveryNode != uuid.Nil {
			return fmt.Errorf("the organization already contains a discovery node, device %s needs to be deleted before adding a new discovery node", existingDiscoveryNode)
		}
	}

	// Relay should never be behind symmetric NAT.
	if nexr.nex.stun && localIP == "" {
		ipPort, err := stunRequest(nexr.logger, stunServer1, nexr.wg.ListenPort)
		if err != nil {
			nexr.logger.Warn("Unable to determine the public facing address, falling back to the local address")
		} else {
			localIP = ipPort.IP.String()
			localEndpointPort = ipPort.Port
		}
	}
	if localIP == "" {
		ip, err := findLocalIP(nexr.logger, nexr.apiserver.controllerURL)
		if err != nil {
			return fmt.Errorf("unable to determine the ip address of the host, please specify using --local-endpoint-ip: %w", err)
		}
		localIP = ip
		localEndpointPort = nexr.wg.ListenPort
	}
	nexr.nex.LocalIP = localIP
	nexr.wg.EndpointLocalAddress = localIP
	endpointSocket := net.JoinHostPort(localIP, fmt.Sprintf("%d", localEndpointPort))
	device, err := nexr.apiserver.CreateDevice(models.AddDevice{
		UserID:                   user.ID,
		OrganizationID:           nexr.nex.organization,
		PublicKey:                nexr.wg.WireguardPubKey,
		LocalIP:                  endpointSocket,
		TunnelIP:                 nexr.nex.requestedIP,
		ChildPrefix:              nil,
		ReflexiveIPv4:            nexr.nex.nodeReflexiveAddress,
		EndpointLocalAddressIPv4: nexr.wg.EndpointLocalAddress,
		SymmetricNat:             false,
		Hostname:                 nexr.nex.hostname,
		Relay:                    nexr.relay,
	})
	if err != nil {
		var conflict client.ErrConflict
		if errors.As(err, &conflict) {
			deviceID, err := uuid.Parse(conflict.ID)
			if err != nil {
				return fmt.Errorf("error parsing conflicting device id: %w", err)
			}
			device, err = nexr.apiserver.UpdateDevice(deviceID, models.UpdateDevice{
				LocalIP:                  endpointSocket,
				ChildPrefix:              nil,
				ReflexiveIPv4:            nexr.nex.nodeReflexiveAddress,
				EndpointLocalAddressIPv4: nexr.wg.EndpointLocalAddress,
				SymmetricNat:             nil,
				Hostname:                 nexr.nex.hostname,
			})
			if err != nil {
				return fmt.Errorf("error updating device: %w", err)
			}
		} else {
			return fmt.Errorf("error creating device: %w", err)
		}
	}
	nexr.logger.Debug(fmt.Sprintf("Device: %+v", device))
	nexr.logger.Infof("Successfully registered device with UUID: %+v", device.ID)

	// a hub router requires ip forwarding and iptables rules, OS type has already been checked
	if err := wireguard.EnableForwardingIPv4(nexr.logger); err != nil {
		return err
	}
	relayIpTables(nexr.logger, nexr.wg.TunnelIface)

	if err := nexr.Reconcile(nexr.nex.organization, true); err != nil {
		return err
	}

	// gather wireguard state from the relay node periodically
	if nexr.discoveryNode {
		util.GoWithWaitGroup(wg, func() {
			util.RunPeriodically(ctx, time.Second*30, func() {
				nexr.logger.Debugf("Reconciling peers from relay state")
				if err := nexr.discoveryStateReconcile(nexr.nex.organization); err != nil {
					nexr.logger.Error(err)
				}
			})
		})
	}

	util.GoWithWaitGroup(wg, func() {
		util.RunPeriodically(ctx, pollInterval, func() {
			if err := nexr.Reconcile(nexr.nex.organization, false); err != nil {
				// TODO: Add smarter reconciliation logic
				nexr.logger.Errorf("Failed to reconcile state with the nexodus API server: %v", err)
				// if the token grant becomes invalid expires refresh or exit depending on the onboard method
				if strings.Contains(err.Error(), invalidTokenGrant.Error()) {
					if nexr.apiserver.username != "" {
						err := nexr.apiserver.Connect(ctx, func(msg string) {
							nexr.SetStatus(NexdStatusAuth, msg)
						})
						if err != nil {
							nexr.logger.Errorf("Failed to reconnect to the api-server, retrying in %v seconds: %v", pollInterval, err)
						} else {
							nexr.SetStatus(NexdStatusRunning, "")
							nexr.logger.Infoln("Nexodus agent has re-established a connection to the api-server")
						}
					} else {
						nexr.logger.Fatalf("The token grant has expired due to an extended period offline, please " +
							"restart the agent for a one-time auth or login with --username --password to automatically reconnect")
					}
				}
			}
		})
	})
	return nil
}

func (nexr *NexRelay) Reconcile(orgID uuid.UUID, firstTime bool) error {
	peerListing, err := nexr.apiserver.GetPeerListing(orgID)
	if err != nil {
		return err
	}
	var newPeers []models.Device
	if firstTime {
		// Initial peer list processing branches from here
		nexr.logger.Debugf("Initializing peers for the first time")
		for _, p := range peerListing {
			existing, ok := nexr.nex.deviceCache[p.ID]
			if !ok {
				nexr.nex.deviceCache[p.ID] = p
				newPeers = append(newPeers, p)
			}
			if !reflect.DeepEqual(existing, p) {
				nexr.nex.deviceCache[p.ID] = p
				newPeers = append(newPeers, p)
			}
		}
		nexr.buildPeersConfig()
		if err := nexr.wg.DeployWireguardConfig(newPeers, firstTime); err != nil {
			if errors.Is(err, errors.New("wireguard config deployment failed")) {
				nexr.logger.Fatal(err)
			}
			return err
		}
	}
	// all subsequent peer listings updates get branched from here
	changed := false
	for _, p := range peerListing {
		existing, ok := nexr.nex.deviceCache[p.ID]
		if !ok {
			changed = true
			nexr.nex.deviceCache[p.ID] = p
			newPeers = append(newPeers, p)
		}
		if !reflect.DeepEqual(existing, p) {
			changed = true
			nexr.nex.deviceCache[p.ID] = p
			newPeers = append(newPeers, p)
		}
	}

	if changed {
		nexr.logger.Debugf("Peers listing has changed, recalculating configuration")
		nexr.buildPeersConfig()
		if err := nexr.wg.DeployWireguardConfig(newPeers, false); err != nil {
			return err
		}
	}

	for _, p := range nexr.nex.deviceCache {
		if wireguard.InPeerListing(peerListing, p) {
			continue
		}
		if err := nexr.wg.HandlePeerDelete(nexr.nex.deviceCache[p.ID]); err != nil {
			return fmt.Errorf("failed to delete peer: %w", err)
		}
		// remove peer from local peer and key cache
		delete(nexr.nex.deviceCache, p.ID)
		delete(nexr.nex.deviceCache, p.ID)
	}

	return nil
}

// discoveryStateReconcile collect state from the discovery node and rejoin nodes with the dynamic state
func (nexr *NexRelay) discoveryStateReconcile(orgID uuid.UUID) error {
	nexr.logger.Debugf("Reconciling peers from relay state")
	peerListing, err := nexr.apiserver.GetPeerListing(orgID)
	if err != nil {
		return err
	}
	// get wireguard state from the discovery node to learn the dynamic reflexive ip:port socket
	discoInfo, err := wireguard.DumpPeers(nexr.wg.TunnelIface)
	if err != nil {
		nexr.logger.Errorf("error dumping wg peers")
	}
	discoData := make(map[string]wireguard.WgSessions)
	for _, peer := range discoInfo {
		_, ok := discoData[peer.PublicKey]
		if !ok {
			discoData[peer.PublicKey] = peer
		}
	}
	// re-join peers with updated state from the discovery node
	for _, peer := range peerListing {
		// if the peer is behind a symmetric NAT, skip to the next peer
		if peer.SymmetricNat {
			nexr.logger.Debugf("skipping symmetric NAT node %s", peer.LocalIP)
			continue
		}
		_, ok := discoData[peer.PublicKey]
		if ok {
			if discoData[peer.PublicKey].Endpoint != "" {
				// test the reflexive address is valid and not still in a (none) state
				_, _, err := net.SplitHostPort(discoData[peer.PublicKey].Endpoint)
				if err != nil {
					// if the discovery state was not yet established or the peer is offline the endpoint can be (none)
					nexr.logger.Debugf("failed to split host:port endpoint pair: %v", err)
					continue
				}
				endpointReflexiveAddress := discoData[peer.PublicKey].Endpoint
				// update the peer endpoint to the new reflexive address learned from the wg session
				_, err = nexr.apiserver.UpdateDevice(peer.ID, models.UpdateDevice{
					LocalIP: endpointReflexiveAddress,
				})
				if err != nil {
					nexr.logger.Errorf("failed updating peer: %+v", err)
				}
			}
		}
	}
	return nil
}

// checkUnsupportedConfigs general matrix checks of required information or constraints to run the agent and join the mesh
func (nexr *NexRelay) checkUnsupportedConfigs() error {
	if nexr.nex.requestedIP != "" {
		nexr.logger.Warnf("request-ip is currently unsupported for the relay node, a dynamic address will be used instead")
		nexr.nex.requestedIP = ""
	}
	return nil
}
