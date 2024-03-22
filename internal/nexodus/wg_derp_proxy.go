package nexodus

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/util"
	"net"
	"net/netip"
	"sync"

	"go.uber.org/zap"
	"go4.org/mem"
	"golang.org/x/net/ipv4"
	"tailscale.com/types/key"
)

// DerpUserSpaceProxy proxies
type DerpUserSpaceProxy struct {
	port       int
	listenAddr net.Addr
	srcAddr    net.Addr
	nexRelay   *nexRelay

	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup

	localConn  net.PacketConn
	packetConn *ipv4.PacketConn
	log        *zap.SugaredLogger
}

// NewWGUserSpaceProxy instantiate a user space WireGuard proxy
func NewDerpUserSpaceProxy(logger *zap.SugaredLogger, nexRelay *nexRelay) *DerpUserSpaceProxy {
	logger.Debugf("Instantiate new userspace derp proxy")
	p := &DerpUserSpaceProxy{
		port:     nexRelay.myDerp,
		log:      logger,
		nexRelay: nexRelay,
	}
	return p
}

func (p *DerpUserSpaceProxy) Restart() {
	p.Stop()

	//Reset the port
	p.mu.Lock()
	p.port = p.nexRelay.myDerp
	p.mu.Unlock()

	p.Start()
}

func (p *DerpUserSpaceProxy) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cancel == nil {
		return
	}

	p.cancel()
	if err := p.closeConn(); err != nil {
		p.log.Errorf("failed to close the local user space connection %v", err)
	}
	p.wg.Wait()
	p.packetConn = nil
	p.localConn = nil
	p.cancel = nil
	p.wg = nil
	p.ctx = nil
	p.listenAddr = nil
}

// Start start the proxy with the given remote conn
func (p *DerpUserSpaceProxy) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.wg = &sync.WaitGroup{}

	// Create a UDP address to listen on
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf(":%d", p.port))
	if err != nil {
		p.log.Errorf("error resolving UDP address: %v", err)
		return
	}

	p.log.Debugf("Listening on UDP address %s", addr.String())

	// Create a UDP connection to listen for incoming packets
	p.localConn, err = net.ListenPacket("udp4", addr.String())
	if err != nil {
		p.log.Errorf("error listening on UDP: %v", err)
		return
	}

	p.packetConn = ipv4.NewPacketConn(p.localConn)
	if err := p.packetConn.SetControlMessage(ipv4.FlagDst, true); err != nil {
		p.log.Errorf("error setting control message: %v", err)
		return
	}

	p.log.Debugf("Proxy start listening on %s for wireguard packets.", p.localConn.LocalAddr())

	p.listenAddr = p.localConn.LocalAddr()
	util.GoWithWaitGroup(p.wg, p.proxyToRemote)
	util.GoWithWaitGroup(p.wg, p.proxyToLocal)
}

// CloseConn close the localConn
func (p *DerpUserSpaceProxy) closeConn() error {
	if p.packetConn != nil {
		return p.packetConn.Close()
	}
	if p.localConn != nil {
		return p.localConn.Close()
	}
	return nil
}

// proxyToRemote proxies everything from Wireguard to the RemoteKey peer
// blocks
func (p *DerpUserSpaceProxy) proxyToRemote() {
	buf := make([]byte, 1500)
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			n, cm, srcAddr, err := p.packetConn.ReadFrom(buf)
			if err != nil {
				p.log.Errorf("Error reading local wireguard UDP packets: %v", err)
				continue
			}

			p.log.Debugf("packet (%d bytes) received from (localAddr : %s, readFromSrcAddr : %s)", n, p.localConn.LocalAddr().String(), srcAddr.String())

			p.srcAddr = srcAddr
			p.log.Debugf("control message: %+v", cm)
			if cm.Dst.IsLoopback() {
				addr, err := netip.ParseAddr(cm.Dst.String())
				if err != nil {
					p.log.Errorf("Error parsing packet destination address: %v", err)
					continue
				}
				addrPort := netip.AddrPortFrom(addr, uint16(p.nexRelay.myDerp))

				pubKeyStr, ok := p.nexRelay.derpIpMapping.GetPublicKey(cm.Dst.String())
				if !ok {
					p.log.Errorf("Error getting public key from derpIpMapping for dst ip %s", cm.Dst)
					continue
				}
				b, err := base64.StdEncoding.DecodeString(pubKeyStr)
				if err != nil {
					p.log.Errorf("Error decoding public key string %s : %v", pubKeyStr, err)
					continue
				}

				pubKey := key.NodePublicFromRaw32(mem.B(b[:]))
				ch := p.nexRelay.derpWriteChanOfAddr(addrPort, pubKey)
				if ch == nil {
					p.log.Errorf("Error getting derp write channel for addr %s", addrPort)
					continue
				}

				select {
				case <-p.ctx.Done():
					return
				case <-p.nexRelay.donec:
					p.log.Errorf("nexRelay is done")
				case ch <- derpWriteRequest{addrPort, pubKey, buf[:n]}:
					p.log.Debugf("packet (%d bytes) sent to (addrPort : %s, pubKey : %s)", n, addrPort, pubKeyStr)
				default:
				}
			}
		}
	}
}

// proxyToLocal proxies everything from the RemoteKey peer to local Wireguard
// blocks
func (p *DerpUserSpaceProxy) proxyToLocal() {
	buf := make([]byte, 1500)
	for {
		select {
		case <-p.ctx.Done():
			return
		case dm := <-p.nexRelay.derpRecvCh:
			if dm.copyBuf == nil {
				p.log.Debugf("No copyBuf func found for the derp read result")
				continue
			}
			ncopy := dm.copyBuf(buf)
			if ncopy != dm.n {
				err := fmt.Errorf("received DERP packet of length %d that's too big for WireGuard buf size %d", dm.n, ncopy)
				p.log.Debugf("derp-read: %v", err)
				continue
			}

			b := dm.src.Raw32() //nolint:staticcheck
			pubKey := base64.StdEncoding.EncodeToString(b[:])
			p.log.Debugf("packet (%d bytes) received from (regionId : %d, wgPubKey: %s pubKey : %s)", ncopy, dm.regionID, dm.src.WireGuardGoString(), pubKey)

			if srcIp := p.nexRelay.derpIpMapping.CheckIfKeyExist(pubKey); srcIp != "" {

				dstIp, _, err := net.SplitHostPort(p.srcAddr.String())
				if err != nil {
					p.log.Debugf("Error splitting host port %s : %v", p.srcAddr.String(), err)
					continue
				}

				cm := &ipv4.ControlMessage{TTL: 0, Src: net.ParseIP(srcIp), Dst: net.ParseIP(dstIp), IfIndex: 1}
				var n int
				n, err = p.packetConn.WriteTo(buf[:ncopy], cm, p.srcAddr)
				if err != nil {
					p.log.Debugf("Error writing to local wireguard UDP packets:%v", err)
					continue
				}
				p.log.Debugf("packet (%d bytes) sent to (writeToDstAddr : %s, writeToSrcAddr : %s)", n, p.srcAddr.String(), cm.Src.String())

			} else {
				p.log.Debugf("Error getting srcIp from derpIpMapping for pubKey %s", pubKey)
				continue
			}
		}
	}
}
