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

type NexAgent struct {
	wg            *wireguard.WireGuard
	apiserver     *ApiServer
	nex           *Nexodus
	childPrefix   []string
	symmetricNat  bool
	logger        *zap.SugaredLogger
	ingresProxies []*UsProxy
	egressProxies []*UsProxy
}

func NewNexAgent(ctx context.Context,
	logger *zap.SugaredLogger,
	controller string,
	username string,
	password string,
	wgListenPort int,
	wireguardPubKey string,
	wireguardPvtKey string,
	requestedIP string,
	userProvidedLocalIP string,
	childPrefix []string,
	stun bool,
	relayOnly bool,
	insecureSkipTlsVerify bool,
	version string,
	userspaceMode bool,
) (*NexAgent, error) {
	wg, err := wireguard.NewWireGuard(wireguardPubKey, wireguardPvtKey, wgListenPort, false, logger, userspaceMode)
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
	nexa := &NexAgent{
		wg:           wg,
		apiserver:    apiserver,
		nex:          nexodus,
		childPrefix:  childPrefix,
		symmetricNat: relayOnly,
		logger:       logger,
	}
	nexa.wg.UserspaceMode = userspaceMode

	// remove orphaned wg interfaces from previous node joins
	nexa.wg.RemoveExistingInterface()

	isSymmetric, reflexiveAddress, err := symmetricNatDisco(nexa.logger, nexa.wg.ListenPort)
	if err != nil {
		nexa.logger.Warn(err)
	}
	nexa.nex.nodeReflexiveAddress = reflexiveAddress
	if isSymmetric {
		nexa.symmetricNat = isSymmetric
		logger.Infof("Symmetric NAT is detected, this node will be provisioned in relay mode only")
	}

	return nexa, nil
}

func (nexa *NexAgent) SetStatus(status int, msg string) {
	nexa.nex.statusMsg = msg
	nexa.nex.status = status
}

func (nexa *NexAgent) Start(ctx context.Context, wg *sync.WaitGroup) error {
	var err error

	// TODO
	//
	// We must deal with a lack of permissions and the possibility for there
	// to be more than one instance of nexd running at the same time in this mode
	// before we can enable it in this mode.
	if !nexa.wg.UserspaceMode {
		if err := nexa.nex.CtlServerStart(ctx, wg); err != nil {
			return fmt.Errorf("CtlServerStart(): %w", err)
		}
	}

	err = nexa.apiserver.Connect(ctx, func(msg string) {
		nexa.SetStatus(NexdStatusAuth, msg)
	})
	if err != nil {
		return err
	}

	nexa.SetStatus(NexdStatusRunning, "")

	if err := nexa.wg.HandleKeys(); err != nil {
		return fmt.Errorf("handleKeys: %w", err)
	}

	user, err := nexa.apiserver.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("get user error: %w", err)
	}

	organizations, err := nexa.apiserver.GetOrganizations()
	if err != nil {
		return fmt.Errorf("get organizations error: %w", err)
	}

	if len(organizations) == 0 {
		return fmt.Errorf("user does not belong to any organizations")
	}
	if len(organizations) != 1 {
		return fmt.Errorf("user being in > 1 organization is not yet supported")
	}
	nexa.logger.Infof("Device belongs in organization: %s (%s)", organizations[0].Name, organizations[0].ID)
	nexa.nex.organization = organizations[0].ID

	var localIP string
	var localEndpointPort int

	// User requested ip --request-ip takes precedent
	if nexa.nex.userProvidedLocalIP != "" {
		localIP = nexa.nex.userProvidedLocalIP
		localEndpointPort = nexa.wg.ListenPort
	}

	// If we are behind a symmetricNat, the endpoint ip discovered by a stun server is useless
	if !nexa.symmetricNat && nexa.nex.stun && localIP == "" {
		ipPort, err := stunRequest(nexa.logger, stunServer1, nexa.wg.ListenPort)
		if err != nil {
			nexa.logger.Warn("Unable to determine the public facing address, falling back to the local address")
		} else {
			localIP = ipPort.IP.String()
			localEndpointPort = ipPort.Port
		}
	}
	if localIP == "" {
		ip, err := findLocalIP(nexa.logger, nexa.apiserver.controllerURL)
		if err != nil {
			return fmt.Errorf("unable to determine the ip address of the host, please specify using --local-endpoint-ip: %w", err)
		}
		localIP = ip
		localEndpointPort = nexa.wg.ListenPort
	}
	nexa.nex.LocalIP = localIP
	nexa.wg.EndpointLocalAddress = localIP
	endpointSocket := net.JoinHostPort(localIP, fmt.Sprintf("%d", localEndpointPort))
	device, err := nexa.apiserver.CreateDevice(models.AddDevice{
		UserID:                   user.ID,
		OrganizationID:           nexa.nex.organization,
		PublicKey:                nexa.wg.WireguardPubKey,
		LocalIP:                  endpointSocket,
		TunnelIP:                 nexa.nex.requestedIP,
		ChildPrefix:              nexa.childPrefix,
		ReflexiveIPv4:            nexa.nex.nodeReflexiveAddress,
		EndpointLocalAddressIPv4: nexa.wg.EndpointLocalAddress,
		SymmetricNat:             nexa.symmetricNat,
		Hostname:                 nexa.nex.hostname,
		Relay:                    false,
	})
	if err != nil {
		var conflict client.ErrConflict
		if errors.As(err, &conflict) {
			deviceID, err := uuid.Parse(conflict.ID)
			if err != nil {
				return fmt.Errorf("error parsing conflicting device id: %w", err)
			}
			device, err = nexa.apiserver.UpdateDevice(deviceID, models.UpdateDevice{
				LocalIP:                  endpointSocket,
				ChildPrefix:              nexa.childPrefix,
				ReflexiveIPv4:            nexa.nex.nodeReflexiveAddress,
				EndpointLocalAddressIPv4: nexa.wg.EndpointLocalAddress,
				SymmetricNat:             &nexa.symmetricNat,
				Hostname:                 nexa.nex.hostname,
			})
			if err != nil {
				return fmt.Errorf("error updating device: %w", err)
			}
		} else {
			return fmt.Errorf("error creating device: %w", err)
		}
	}
	nexa.logger.Debug(fmt.Sprintf("Device: %+v", device))
	nexa.logger.Infof("Successfully registered device with UUID: %+v", device.ID)

	if err := nexa.Reconcile(nexa.nex.organization, true); err != nil {
		return err
	}

	// Agent sends keepalives to all peers periodically
	util.GoWithWaitGroup(wg, func() {
		util.RunPeriodically(ctx, time.Second*10, func() {
			nexa.keepalive()
		})
	})

	util.GoWithWaitGroup(wg, func() {
		util.RunPeriodically(ctx, pollInterval, func() {
			if err := nexa.Reconcile(nexa.nex.organization, false); err != nil {
				// TODO: Add smarter reconciliation logic
				nexa.logger.Errorf("Failed to reconcile state with the nexodus API server: %v", err)
				// if the token grant becomes invalid expires refresh or exit depending on the onboard method
				if strings.Contains(err.Error(), invalidTokenGrant.Error()) {
					if nexa.apiserver.username != "" {
						err := nexa.apiserver.Connect(ctx, func(msg string) {
							nexa.SetStatus(NexdStatusAuth, msg)
						})
						if err != nil {
							nexa.logger.Errorf("Failed to reconnect to the api-server, retrying in %v seconds: %v", pollInterval, err)
						} else {
							nexa.SetStatus(NexdStatusRunning, "")
							nexa.logger.Infoln("Nexodus agent has re-established a connection to the api-server")
						}
					} else {
						nexa.logger.Fatalf("The token grant has expired due to an extended period offline, please " +
							"restart the agent for a one-time auth or login with --username --password to automatically reconnect")
					}
				}
			}
		})
	})

	for _, proxy := range nexa.ingresProxies {
		proxy.Start(ctx, wg, nexa.wg.UserspaceNet)
	}
	for _, proxy := range nexa.egressProxies {
		proxy.Start(ctx, wg, nexa.wg.UserspaceNet)
	}

	return nil
}

func (nexa *NexAgent) keepalive() {
	if nexa.wg.UserspaceMode {
		nexa.logger.Debugf("Keepalive not yet implemented in userspace mode")
		return
	}
	nexa.logger.Debug("Sending Keepalive")
	var peerEndpoints []string
	for _, value := range nexa.nex.deviceCache {
		nodeAddr := value.TunnelIP
		// strip the /32 from the prefix if present
		if net.ParseIP(value.TunnelIP) == nil {
			nodeIP, _, err := net.ParseCIDR(value.TunnelIP)
			nodeAddr = nodeIP.String()
			if err != nil {
				nexa.logger.Debugf("failed parsing an ip from the prefix %v", err)
			}
		}
		peerEndpoints = append(peerEndpoints, nodeAddr)
	}

	_ = probePeers(peerEndpoints, nexa.logger)
}

func (nexa *NexAgent) Reconcile(orgID uuid.UUID, firstTime bool) error {
	peerListing, err := nexa.apiserver.GetPeerListing(orgID)
	if err != nil {
		return err
	}
	var newPeers []models.Device
	if firstTime {
		// Initial peer list processing branches from here
		nexa.logger.Debugf("Initializing peers for the first time")
		for _, p := range peerListing {
			existing, ok := nexa.nex.deviceCache[p.ID]
			if !ok {
				nexa.nex.deviceCache[p.ID] = p
				newPeers = append(newPeers, p)
			}
			if !reflect.DeepEqual(existing, p) {
				nexa.nex.deviceCache[p.ID] = p
				newPeers = append(newPeers, p)
			}
		}
		nexa.buildPeersConfig()
		if err := nexa.wg.DeployWireguardConfig(newPeers, firstTime); err != nil {
			if errors.Is(err, errors.New("wireguard config deployment failed")) {
				nexa.logger.Fatal(err)
			}
			return err
		}
	}
	// all subsequent peer listings updates get branched from here
	changed := false
	for _, p := range peerListing {
		existing, ok := nexa.nex.deviceCache[p.ID]
		if !ok {
			changed = true
			nexa.nex.deviceCache[p.ID] = p
			newPeers = append(newPeers, p)
		}
		if !reflect.DeepEqual(existing, p) {
			changed = true
			nexa.nex.deviceCache[p.ID] = p
			newPeers = append(newPeers, p)
		}
	}

	if changed {
		nexa.logger.Debugf("Peers listing has changed, recalculating configuration")
		nexa.buildPeersConfig()
		if err := nexa.wg.DeployWireguardConfig(newPeers, false); err != nil {
			return err
		}
	}

	for _, p := range nexa.nex.deviceCache {
		if wireguard.InPeerListing(peerListing, p) {
			continue
		}
		if err := nexa.wg.HandlePeerDelete(nexa.nex.deviceCache[p.ID]); err != nil {
			return fmt.Errorf("failed to delete peer: %w", err)
		}
		// remove peer from local peer and key cache
		delete(nexa.nex.deviceCache, p.ID)
		delete(nexa.nex.deviceCache, p.ID)
	}

	return nil
}
