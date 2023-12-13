package nexodus

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/netip"
	"reflect"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/nexodus-io/nexodus/internal/nexodus/derp/derphttp"
	"tailscale.com/derp"
	"tailscale.com/health"
	"tailscale.com/logtail/backoff"
	"tailscale.com/net/dnscache"
	"tailscale.com/syncs"
	"tailscale.com/tailcfg"
	"tailscale.com/types/key"
	"tailscale.com/types/logger"
	"tailscale.com/util/mak"
	"tailscale.com/util/sysresources"
)

const (
	DefaultDerpRegionID   = 900
	DefaultDerpRegionCode = "web"
	DefaultDerpRegionName = "NexodusDefault"
	DefaultDerpNodeName   = "900nex"
	DefaultDerpIPAddr     = "relay.nexodus.io"
	CustomDerpRegionID    = 901
	CustomDerpRegionCode  = "local"
	CustomDerpRegionName  = "NexodusLocal"
	CustomDerpNodeName    = "901nex"
	CustomDerpIPAddr      = "custom.relay.nexodus.io"
)

// derpRoute is a route entry for a public key, saying that a certain
// peer should be available at DERP node derpID, as long as the
// current connection for that derpID is dc. (but dc should not be
// used to write directly; it's owned by the read/write loops)
type derpRoute struct {
	derpID int
	dc     *derphttp.Client // don't use directly; see comment above
}

// removeDerpPeerRoute removes a DERP route entry previously added by addDerpPeerRoute.
func (nr *nexRelay) removeDerpPeerRoute(peer key.NodePublic, derpID int, dc *derphttp.Client) {
	nr.mu.Lock()
	defer nr.mu.Unlock()
	r2 := derpRoute{derpID, dc}
	if r, ok := nr.derpRoute[peer]; ok && r == r2 {
		delete(nr.derpRoute, peer)
	}
}

// addDerpPeerRoute adds a DERP route entry, noting that peer was seen
// on DERP node derpID, at least on the connection identified by dc.
// See issue 150 for details.
func (nr *nexRelay) addDerpPeerRoute(peer key.NodePublic, derpID int, dc *derphttp.Client) {
	nr.mu.Lock()
	defer nr.mu.Unlock()
	mak.Set(&nr.derpRoute, peer, derpRoute{derpID, dc})
}

// activeDerp contains fields for an active DERP connection.
type activeDerp struct {
	c       *derphttp.Client
	cancel  context.CancelFunc
	writeCh chan<- derpWriteRequest
	// lastWrite is the time of the last request for its write
	// channel (currently even if there was no write).
	// It is always non-nil and initialized to a non-zero Time.
	lastWrite  *time.Time
	createTime time.Time
}

var processStartUnixNano = time.Now().UnixNano()

func (nr *nexRelay) derpRegionCodeLocked(regionID int) string {
	if nr.derpMap == nil {
		return ""
	}
	if dr, ok := nr.derpMap.Regions[regionID]; ok {
		return dr.RegionCode
	}
	return ""
}

// c.mu must NOT be held.
func (nr *nexRelay) setNearestDERP(derpNum int) (wantDERP bool) {
	nr.mu.Lock()
	defer nr.mu.Unlock()
	if !nr.wantDerpLocked() {
		nr.myDerp = 0
		health.SetMagicSockDERPHome(0)
		return false
	}
	if derpNum == nr.myDerp {
		// No change.
		return true
	}
	nr.myDerp = derpNum
	health.SetMagicSockDERPHome(derpNum)

	if nr.privateKey.IsZero() {
		// No private key yet, so DERP connections won't come up anyway.
		// Return early rather than ultimately log a couple lines of noise.
		return true
	}

	// On change, notify all currently connected DERP servers and
	// start connecting to our home DERP if we are not already.
	dr := nr.derpMap.Regions[derpNum]
	if dr == nil {
		nr.logger.Errorf("[derpMap regions for derp region code [%d] is nil", derpNum)
	} else {
		nr.logger.Infof("home region for %d is now derp-%v (%s)", derpNum, nr.derpMap.Regions[derpNum].RegionCode)
	}
	for i, ad := range nr.activeDerp {
		go ad.c.NotePreferred(i == nr.myDerp)
	}
	nr.goDerpConnect(derpNum)
	return true
}

// startDerpHomeConnectLocked starts connecting to our DERP home, if any.
//
// c.mu must be held.
func (nr *nexRelay) startDerpHomeConnectLocked() {
	nr.goDerpConnect(nr.myDerp)
}

// goDerpConnect starts a goroutine to start connecting to the given
// DERP node.
//
// c.mu may be held, but does not need to be.
func (nr *nexRelay) goDerpConnect(node int) {
	if node == 0 {
		return
	}
	go nr.derpWriteChanOfAddr(netip.AddrPortFrom(tailcfg.DerpMagicIPAddr, uint16(node)), key.NodePublic{})
}

var (
	bufferedDerpWrites     int
	bufferedDerpWritesOnce sync.Once
)

// bufferedDerpWritesBeforeDrop returns how many packets writes can be queued
// up the DERP client to write on the wire before we start dropping.
func bufferedDerpWritesBeforeDrop() int {
	// For mobile devices, always return the previous minimum value of 32;
	// we can do this outside the sync.Once to avoid that overhead.
	if runtime.GOOS == "ios" || runtime.GOOS == "android" {
		return 32
	}

	bufferedDerpWritesOnce.Do(func() {
		// Some rough sizing: for the previous fixed value of 32, the
		// total consumed memory can be:
		// = numDerpRegions * messages/region * sizeof(message)
		//
		// For sake of this calculation, assume 100 DERP regions; at
		// time of writing (2023-04-03), we have 24.
		//
		// A reasonable upper bound for the worst-case average size of
		// a message is a *disco.CallMeMaybe message with 16 endpoints;
		// since sizeof(netip.AddrPort) = 32, that's 512 bytes. Thus:
		// = 100 * 32 * 512
		// = 1638400 (1.6MiB)
		//
		// On a reasonably-small node with 4GiB of memory that's
		// connected to each region and handling a lot of load, 1.6MiB
		// is about 0.04% of the total system memory.
		//
		// For sake of this calculation, then, let's double that memory
		// usage to 0.08% and scale based on total system memory.
		//
		// For a 16GiB Linux box, this should buffer just over 256
		// messages.
		systemMemory := sysresources.TotalMemory()
		memoryUsable := float64(systemMemory) * 0.0008

		const (
			theoreticalDERPRegions  = 100
			messageMaximumSizeBytes = 512
		)
		bufferedDerpWrites = int(memoryUsable / (theoreticalDERPRegions * messageMaximumSizeBytes))

		// Never drop below the previous minimum value.
		if bufferedDerpWrites < 32 {
			bufferedDerpWrites = 32
		}
	})
	return bufferedDerpWrites
}

func (nr *nexRelay) networkDown() bool { return false }

// derpWriteChanOfAddr returns a DERP client for fake UDP addresses that
// represent DERP servers, creating them as necessary. For real UDP
// addresses, it returns nil.
//
// If peer is non-zero, it can be used to find an active reverse
// path, without using addr.
func (nr *nexRelay) derpWriteChanOfAddr(addr netip.AddrPort, peer key.NodePublic) chan<- derpWriteRequest {
	regionID := int(addr.Port())

	if nr.networkDown() {
		return nil
	}

	nr.mu.Lock()
	defer nr.mu.Unlock()
	if !nr.wantDerpLocked() || nr.closed {
		return nil
	}
	if nr.derpMap == nil || nr.derpMap.Regions[regionID] == nil {
		return nil
	}
	if nr.privateKey.IsZero() {
		nr.logger.Debugf("DERP lookup of %v with no private key; ignoring", addr)
		return nil
	}

	// See if we have a connection open to that DERP node ID
	// first. If so, might as well use it. (It's a little
	// arbitrary whether we use this one vs. the reverse route
	// below when we have both.)
	ad, ok := nr.activeDerp[regionID]
	if ok {
		*ad.lastWrite = time.Now()
		nr.setPeerLastDerpLocked(peer, regionID, regionID)
		return ad.writeCh
	}

	why := "home-keep-alive"
	if !peer.IsZero() {
		why = peer.ShortString()
	}
	nr.logger.Infof("adding connection to derp-%d for %s", regionID, why)

	firstDerp := false
	if nr.activeDerp == nil {
		firstDerp = true
		nr.activeDerp = make(map[int]activeDerp)
		nr.prevDerp = make(map[int]*syncs.WaitGroupChan)
	}

	// Note that derphttp.NewRegionClient does not dial the server
	// (it doesn't block) so it is safe to do under the c.mu lock.
	dc := derphttp.NewRegionClient(nr.privateKey, nr.logf, nil, func() *tailcfg.DERPRegion {
		// Warning: it is not legal to acquire
		// magicsock.Conn.mu from this callback.
		// It's run from derphttp.Client.connect (via Send, etc)
		// and the lock ordering rules are that magicsock.Conn.mu
		// must be acquired before derphttp.Client.mu.
		// See https://github.com/tailscale/tailscale/issues/3726
		if nr.connCtx.Err() != nil {
			// We're closing anyway; return nil to stop dialing.
			return nil
		}
		derpMap := nr.derpMapAtomic.Load()
		if derpMap == nil {
			return nil
		}
		return derpMap.Regions[regionID]
	})

	dc.SetCanAckPings(true)
	dc.NotePreferred(nr.myDerp == regionID)
	dc.SetAddressFamilySelector(derpAddrFamSelector{nr})
	dc.DNSCache = dnscache.Get()

	ctx, cancel := context.WithCancel(nr.connCtx)
	ch := make(chan derpWriteRequest, bufferedDerpWritesBeforeDrop())

	ad.c = dc
	ad.writeCh = ch
	ad.cancel = cancel
	ad.lastWrite = new(time.Time)
	*ad.lastWrite = time.Now()
	ad.createTime = time.Now()
	nr.activeDerp[regionID] = ad
	nr.logActiveDerpLocked()
	nr.setPeerLastDerpLocked(peer, regionID, regionID)

	// Build a startGate for the derp reader+writer
	// goroutines, so they don't start running until any
	// previous generation is closed.
	startGate := syncs.ClosedChan()
	if prev := nr.prevDerp[regionID]; prev != nil {
		startGate = prev.DoneChan()
	}
	// And register a WaitGroup(Chan) for this generation.
	wg := syncs.NewWaitGroupChan()
	wg.Add(2)
	nr.prevDerp[regionID] = wg

	if firstDerp {
		startGate = nr.derpStarted
		go func() {
			dc.Connect(ctx)
			close(nr.derpStarted)
			nr.muCond.Broadcast()
		}()
	}

	go nr.runDerpReader(ctx, addr, dc, wg, startGate)
	go nr.runDerpWriter(ctx, dc, ch, wg, startGate)

	return ad.writeCh
}

// setPeerLastDerpLocked notes that peer is now being written to via
// the provided DERP regionID, and that the peer advertises a DERP
// home region ID of homeID.
//
// If there's any change, it logs.
//
// nr.mu must be held.
func (nr *nexRelay) setPeerLastDerpLocked(peer key.NodePublic, regionID, homeID int) {
	if peer.IsZero() {
		return
	}
	old := nr.peerLastDerp[peer]
	if old == regionID {
		return
	}
	nr.peerLastDerp[peer] = regionID

	var newDesc string
	switch {
	case regionID == homeID && regionID == nr.myDerp:
		newDesc = "shared home"
	case regionID == homeID:
		newDesc = "their home"
	case regionID == nr.myDerp:
		newDesc = "our home"
	case regionID != homeID:
		newDesc = "alt"
	}
	if old == 0 {
		nr.logger.Infof("[v1] derp route for %s set to derp-%d (%s)", peer.ShortString(), regionID, newDesc)
	} else {
		nr.logger.Infof("[v1] derp route for %s changed from derp-%d => derp-%d (%s)", peer.ShortString(), old, regionID, newDesc)
	}
}

// derpReadResult is the type sent by runDerpClient to ReceiveIPv4
// when a DERP packet is available.
//
// Notably, it doesn't include the derp.ReceivedPacket because we
// don't want to give the receiver access to the aliased []byte.  To
// get at the packet contents they need to call copyBuf to copy it
// out, which also releases the buffer.
type derpReadResult struct {
	regionID int
	n        int // length of data received
	src      key.NodePublic
	// copyBuf is called to copy the data to dst.  It returns how
	// much data was copied, which will be n if dst is large
	// enough. copyBuf can only be called once.
	// If copyBuf is nil, that's a signal from the sender to ignore
	// this message.
	copyBuf func(dst []byte) int
}

// runDerpReader runs in a goroutine for the life of a DERP
// connection, handling received packets.
func (nr *nexRelay) runDerpReader(ctx context.Context, derpFakeAddr netip.AddrPort, dc *derphttp.Client, wg *syncs.WaitGroupChan, startGate <-chan struct{}) {
	defer wg.Decr()
	defer dc.Close()

	select {
	case <-startGate:
	case <-ctx.Done():
		return
	}

	didCopy := make(chan struct{}, 1)
	regionID := int(derpFakeAddr.Port())
	res := derpReadResult{regionID: regionID}
	var pkt derp.ReceivedPacket
	res.copyBuf = func(dst []byte) int {
		n := copy(dst, pkt.Data)
		didCopy <- struct{}{}
		return n
	}

	defer health.SetDERPRegionConnectedState(regionID, false)
	defer health.SetDERPRegionHealth(regionID, "")

	// peerPresent is the set of senders we know are present on this
	// connection, based on messages we've received from the server.
	peerPresent := map[key.NodePublic]bool{}
	bo := backoff.NewBackoff(fmt.Sprintf("derp-%d", regionID), nr.logf, 5*time.Second)
	var lastPacketTime time.Time
	var lastPacketSrc key.NodePublic

	for {
		msg, connGen, err := dc.RecvDetail()
		if err != nil {
			health.SetDERPRegionConnectedState(regionID, false)
			// Forget that all these peers have routes.
			for peer := range peerPresent {
				delete(peerPresent, peer)
				nr.removeDerpPeerRoute(peer, regionID, dc)
			}
			if err == derphttp.ErrClientClosed {
				return
			}
			if nr.networkDown() {
				nr.logger.Warnf("[v1] derp.Recv(derp-%d): network down, closing", regionID)
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
			}

			nr.logger.Infof("[%p] derp.Recv(derp-%d): %v", dc, regionID, err)

			// Back off a bit before reconnecting.
			bo.BackOff(ctx, err)
			select {
			case <-ctx.Done():
				return
			default:
			}
			continue
		}
		bo.BackOff(ctx, nil) // reset

		now := time.Now()
		if lastPacketTime.IsZero() || now.Sub(lastPacketTime) > 5*time.Second {
			health.NoteDERPRegionReceivedFrame(regionID)
			lastPacketTime = now
		}

		switch m := msg.(type) {
		case derp.ServerInfoMessage:
			health.SetDERPRegionConnectedState(regionID, true)
			health.SetDERPRegionHealth(regionID, "") // until declared otherwise
			nr.logger.Infof("derp-%d connected; connGen=%v", regionID, connGen)
			continue
		case derp.ReceivedPacket:
			pkt = m
			res.n = len(m.Data)
			res.src = m.Source
			nr.logger.Debugf("got derp-%d packet of len : %d received", regionID, res.n)

			// If this is a new sender we hadn't seen before, remember it and
			// register a route for this peer.
			if res.src != lastPacketSrc { // avoid map lookup w/ high throughput single peer
				lastPacketSrc = res.src
				if _, ok := peerPresent[res.src]; !ok {
					peerPresent[res.src] = true
					nr.addDerpPeerRoute(res.src, regionID, dc)
				}
			}
		case derp.PingMessage:
			// Best effort reply to the ping.
			pingData := [8]byte(m)
			go func() {
				if err := dc.SendPong(pingData); err != nil {
					nr.logger.Errorf("derp-%d SendPong error: %v", regionID, err)
				}
			}()
			continue
		case derp.HealthMessage:
			health.SetDERPRegionHealth(regionID, m.Problem)
		case derp.PeerGoneMessage:
			switch m.Reason {
			case derp.PeerGoneReasonDisconnected:
				// Do nothing.
			case derp.PeerGoneReasonNotHere:
				nr.logger.Infof("[unexpected] derp-%d does not know about peer %s, removing route",
					regionID, key.NodePublic(m.Peer).ShortString())
			default:
				nr.logger.Infof("[unexpected] derp-%d peer %s gone, reason %v, removing route",
					regionID, key.NodePublic(m.Peer).ShortString(), m.Reason)
			}
			nr.removeDerpPeerRoute(key.NodePublic(m.Peer), regionID, dc)
		default:
			// Ignore.
			continue
		}

		select {
		case <-ctx.Done():
			return
		case nr.derpRecvCh <- res:
		}

		select {
		case <-ctx.Done():
			return
		case <-didCopy:
			continue
		}
	}
}

type derpWriteRequest struct {
	addr   netip.AddrPort
	pubKey key.NodePublic
	b      []byte // copied; ownership passed to receiver
}

// runDerpWriter runs in a goroutine for the life of a DERP
// connection, handling received packets.
func (nr *nexRelay) runDerpWriter(ctx context.Context, dc *derphttp.Client, ch <-chan derpWriteRequest, wg *syncs.WaitGroupChan, startGate <-chan struct{}) {
	defer wg.Decr()
	select {
	case <-startGate:
	case <-ctx.Done():
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case wr := <-ch:
			err := dc.Send(wr.pubKey, wr.b)
			if err != nil {
				nr.logger.Errorf("packet send to derp server (%v) failed: %v", wr.addr, err)
			}
		}
	}
}

func (nr *nexRelay) receiveDERP(buffs [][]byte, sizes []int) (int, error) {
	for dm := range nr.derpRecvCh {
		n := nr.processDERPReadResult(dm, buffs[0])
		if n == 0 {
			// No data read occurred. Wait for another packet.
			continue
		}
		sizes[0] = n
		return 1, nil
	}
	return 0, net.ErrClosed
}

func (nr *nexRelay) processDERPReadResult(dm derpReadResult, b []byte) (n int) {
	if dm.copyBuf == nil {
		return 0
	}
	ncopy := dm.copyBuf(b)
	if ncopy != dm.n {
		err := fmt.Errorf("received DERP packet of length %d that's too big for WireGuard buf size %d", dm.n, ncopy)
		nr.logger.Errorf("failed to read derp read results: %v", err)
		return 0
	}
	return n
}

func (nr *nexRelay) debugUseDERPAddr() string {
	return DefaultDerpIPAddr
}

func (nr *nexRelay) debugUseDERPHTTP() bool {
	return false
}

func (nr *nexRelay) debugUseDERP() bool {
	return nr.debugUseDERPAddr() != ""
}

// SetDefaultDERPMap sets the default DERP map to use for nexodus deployments
func (nr *nexRelay) SetCustomDERPMap(derpStunAddr string, hostname string) {
	nr.mu.Lock()
	defer nr.mu.Unlock()
	var dm *tailcfg.DERPMap
	derpPort := 443
	if nr.debugUseDERPHTTP() {
		// Match the port for -dev in derper.go
		derpPort = 3340
	}
	dm = &tailcfg.DERPMap{
		OmitDefaultRegions: true,
		Regions: map[int]*tailcfg.DERPRegion{
			CustomDerpRegionID: {
				RegionID:   CustomDerpRegionID,
				RegionName: CustomDerpRegionName,
				RegionCode: CustomDerpRegionCode,
				Nodes: []*tailcfg.DERPNode{{
					Name:     CustomDerpNodeName,
					RegionID: CustomDerpRegionID,
					HostName: hostname,
					DERPPort: derpPort,
				}},
			},
		},
	}

	nr.derpMapAtomic.Store(dm)
	nr.derpMap = dm
	nr.myDerp = CustomDerpRegionID
}

// SetDefaultDERPMap sets the default DERP map to use for nexodus deployments
func (nr *nexRelay) SetDefaultDERPMap() {
	nr.mu.Lock()
	defer nr.mu.Unlock()
	var dm *tailcfg.DERPMap
	var derpAddr = nr.debugUseDERPAddr()
	if derpAddr != "" {
		derpPort := 443
		if nr.debugUseDERPHTTP() {
			// Match the port for -dev in derper.go
			derpPort = 3340
		}
		dm = &tailcfg.DERPMap{
			OmitDefaultRegions: true,
			Regions: map[int]*tailcfg.DERPRegion{
				DefaultDerpRegionID: {
					RegionID:   DefaultDerpRegionID,
					RegionName: DefaultDerpRegionName,
					RegionCode: DefaultDerpRegionCode,
					Nodes: []*tailcfg.DERPNode{{
						Name:     DefaultDerpNodeName,
						RegionID: DefaultDerpRegionID,
						HostName: derpAddr,
						DERPPort: derpPort,
					}},
				},
			},
		}
	}

	nr.derpMapAtomic.Store(dm)
	nr.derpMap = dm
	nr.myDerp = DefaultDerpRegionID
}

func (nr *nexRelay) UnsetDefaultDERPMap() {
	nr.mu.Lock()
	defer nr.mu.Unlock()
	nr.derpMapAtomic.Store(nil)
	nr.derpMap = nil
	nr.myDerp = 0
}

// SetDERPMap controls which (if any) DERP servers are used.
// A nil value means to disable DERP; it's disabled by default.
func (nr *nexRelay) SetDERPMap(dm *tailcfg.DERPMap) {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	var derpAddr = nr.debugUseDERPAddr()
	if derpAddr != "" {
		derpPort := 443
		if nr.debugUseDERPHTTP() {
			// Match the port for -dev in derper.go
			derpPort = 3340
		}
		dm = &tailcfg.DERPMap{
			OmitDefaultRegions: true,
			Regions: map[int]*tailcfg.DERPRegion{
				900: {
					RegionID: 900,
					Nodes: []*tailcfg.DERPNode{{
						Name:     "nexodus",
						RegionID: 900,
						HostName: derpAddr,
						DERPPort: derpPort,
					}},
				},
			},
		}
	}

	if reflect.DeepEqual(dm, nr.derpMap) {
		return
	}

	nr.derpMapAtomic.Store(dm)
	old := nr.derpMap
	nr.derpMap = dm
	if dm == nil {
		nr.closeAllDerpLocked("derp-disabled")
		return
	}

	// Reconnect any DERP region that changed definitions.
	if old != nil {
		changes := false
		for rid, oldDef := range old.Regions {
			if reflect.DeepEqual(oldDef, dm.Regions[rid]) {
				continue
			}
			changes = true
			if rid == nr.myDerp {
				nr.myDerp = 0
			}
			nr.closeDerpLocked(rid, "derp-region-redefined")
		}
		if changes {
			nr.logActiveDerpLocked()
		}
	}
}

func (nr *nexRelay) getDerpRelayHostname(regionId int) (string, error) {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	if nr.derpMap != nil {
		region, ok := nr.derpMap.Regions[regionId]
		if !ok {
			return "", fmt.Errorf("region %d not found in derp map", regionId)
		}

		if len(region.Nodes) == 0 {
			return "", fmt.Errorf("region %d has no registered derp relay nodes", regionId)
		}

		return region.Nodes[0].HostName, nil
	}

	return "", fmt.Errorf("derp map is nil")
}

func (nr *nexRelay) wantDerpLocked() bool { return nr.derpMap != nil }

// c.mu must be held.
func (nr *nexRelay) closeAllDerpLocked(why string) {
	if len(nr.activeDerp) == 0 {
		return // without the useless log statement
	}
	for i := range nr.activeDerp {
		nr.closeDerpLocked(i, why)
	}
	nr.logActiveDerpLocked()
}

// DebugBreakDERPConns breaks all DERP connections for debug/testing reasons.
func (nr *nexRelay) DebugBreakDERPConns() error {
	nr.mu.Lock()
	defer nr.mu.Unlock()
	if len(nr.activeDerp) == 0 {
		nr.logger.Info("DebugBreakDERPConns: no active DERP connections")
		return nil
	}
	nr.closeAllDerpLocked("debug-break-derp")
	nr.startDerpHomeConnectLocked()
	return nil
}

// closeOrReconnectDERPLocked closes the DERP connection to the
// provided regionID and starts reconnecting it if it's our current
// home DERP.
//
// why is a reason for logging.
//
// c.mu must be held.
func (nr *nexRelay) closeOrReconnectDERPLocked(regionID int, why string) {
	nr.closeDerpLocked(regionID, why)
	if !nr.privateKey.IsZero() && nr.myDerp == regionID {
		nr.startDerpHomeConnectLocked()
	}
}

// c.mu must be held.
// It is the responsibility of the caller to call logActiveDerpLocked after any set of closes.
func (nr *nexRelay) closeDerpLocked(regionID int, why string) {
	if ad, ok := nr.activeDerp[regionID]; ok {
		nr.logger.Infof("closing connection to derp-%d (%s), age %v", regionID, why, time.Since(ad.createTime).Round(time.Second))
		go ad.c.Close()
		ad.cancel()
		delete(nr.activeDerp, regionID)
	}
}

// simpleDur rounds d such that it stringifies to something short.
func simpleDur(d time.Duration) time.Duration {
	if d < time.Second {
		return d.Round(time.Millisecond)
	}
	if d < time.Minute {
		return d.Round(time.Second)
	}
	return d.Round(time.Minute)
}

// c.mu must be held.
func (nr *nexRelay) logActiveDerpLocked() {
	now := time.Now()
	nr.logger.Infof("%d active derp conns %s", len(nr.activeDerp), logger.ArgWriter(func(buf *bufio.Writer) {
		if len(nr.activeDerp) == 0 {
			return
		}
		buf.WriteString(":")
		nr.foreachActiveDerpSortedLocked(func(node int, ad activeDerp) {
			fmt.Fprintf(buf, " derp-%d=cr%v,wr%v", node, simpleDur(now.Sub(ad.createTime)), simpleDur(now.Sub(*ad.lastWrite)))
		})
	}))
}

// c.mu must be held.
func (nr *nexRelay) foreachActiveDerpSortedLocked(fn func(regionID int, ad activeDerp)) {
	if len(nr.activeDerp) < 2 {
		for id, ad := range nr.activeDerp {
			fn(id, ad)
		}
		return
	}
	ids := make([]int, 0, len(nr.activeDerp))
	for id := range nr.activeDerp {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		fn(id, nr.activeDerp[id])
	}
}

func (nr *nexRelay) cleanStaleDerp() {
	nr.mu.Lock()
	defer nr.mu.Unlock()
	if nr.closed {
		return
	}
	nr.derpCleanupTimerArmed = false

	tooOld := time.Now().Add(-derpInactiveCleanupTime)
	dirty := false
	someNonHomeOpen := false
	for i, ad := range nr.activeDerp {
		if i == nr.myDerp {
			continue
		}
		if ad.lastWrite.Before(tooOld) {
			nr.closeDerpLocked(i, "idle")
			dirty = true
		} else {
			someNonHomeOpen = true
		}
	}
	if dirty {
		nr.logActiveDerpLocked()
	}
	if someNonHomeOpen {
		nr.scheduleCleanStaleDerpLocked()
	}
}

func (nr *nexRelay) scheduleCleanStaleDerpLocked() {
	if nr.derpCleanupTimerArmed {
		// Already going to fire soon. Let the existing one
		// fire lest it get infinitely delayed by repeated
		// calls to scheduleCleanStaleDerpLocked.
		return
	}
	nr.derpCleanupTimerArmed = true
	if nr.derpCleanupTimer != nil {
		nr.derpCleanupTimer.Reset(derpCleanStaleInterval)
	} else {
		nr.derpCleanupTimer = time.AfterFunc(derpCleanStaleInterval, nr.cleanStaleDerp)
	}
}

// DERPs reports the number of active DERP connections.
func (nr *nexRelay) DERPs() int {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	return len(nr.activeDerp)
}

func (nr *nexRelay) derpRegionCodeOfIDLocked(regionID int) string {
	if nr.derpMap == nil {
		return ""
	}
	if r, ok := nr.derpMap.Regions[regionID]; ok {
		return r.RegionCode
	}
	return ""
}

// derpAddrFamSelector is the derphttp.AddressFamilySelector we pass
// to derphttp.Client.SetAddressFamilySelector.
//
// It provides the hint as to whether in an IPv4-vs-IPv6 race that
// IPv4 should be held back a bit to give IPv6 a better-than-50/50
// chance of winning. We only return true when we believe IPv6 will
// work anyway, so we don't artificially delay the connection speed.
type derpAddrFamSelector struct{ c *nexRelay }

func (s derpAddrFamSelector) PreferIPv6() bool {
	return false
}

const (
	// derpInactiveCleanupTime is how long a non-home DERP connection
	// needs to be idle (last written to) before we close it.
	derpInactiveCleanupTime = 60 * time.Second

	// derpCleanStaleInterval is how often cleanStaleDerp runs when there
	// are potentially-stale DERP connections to close.
	derpCleanStaleInterval = 15 * time.Second
)
