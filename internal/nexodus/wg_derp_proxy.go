package nexodus

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/netip"

	"go.uber.org/zap"
	"go4.org/mem"
	"golang.org/x/net/ipv4"
	"tailscale.com/types/key"
)

// WGUserSpaceProxy proxies
type WGUserSpaceProxy struct {
	port     int
	srcAddr  net.Addr
	nexRelay *nexRelay
	ctx      context.Context
	cancel   context.CancelFunc

	remoteConn net.Conn
	localConn  net.PacketConn
	packetConn *ipv4.PacketConn
	log        *zap.SugaredLogger
}

func StartDerpProxy(logger *zap.SugaredLogger, nexRelay *nexRelay) *WGUserSpaceProxy {
	derpProxy := newWGUserSpaceProxy(logger, nexRelay)

	derpProxy.StartListening(nil)
	return derpProxy
}

func (p *WGUserSpaceProxy) StopDerpProxy() {
	p.ctx.Done()
}

// NewWGUserSpaceProxy instantiate a user space WireGuard proxy
func newWGUserSpaceProxy(logger *zap.SugaredLogger, nexRelay *nexRelay) *WGUserSpaceProxy {
	logger.Debugf("instantiate new userspace derp proxy")
	p := &WGUserSpaceProxy{
		port:     nexRelay.myDerp,
		log:      logger,
		nexRelay: nexRelay,
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	return p
}

// StartListening start the proxy with the given remote conn
func (p *WGUserSpaceProxy) StartListening(remoteConn net.Conn) (net.Addr, error) {
	// Create a UDP address to listen on
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf(":%d", p.port))
	if err != nil {
		p.log.Errorf("Error resolving UDP address:", err)
		return nil, err
	}

	p.log.Debugf("addr is ", addr.IP, addr.Port)

	// Create a UDP connection to listen for incoming packets
	p.localConn, err = net.ListenPacket("udp4", addr.String())
	if err != nil {
		p.log.Errorf("Error listening on UDP:", err)
		return nil, err
	}

	p.packetConn = ipv4.NewPacketConn(p.localConn)
	if err := p.packetConn.SetControlMessage(ipv4.FlagDst, true); err != nil {
		p.log.Errorf("Error setting control message:", err)
		return nil, err
	}

	//print conn4
	p.log.Debugf("Proxy start listening on %s for wireguard packets.", p.localConn.LocalAddr())

	go p.proxyToRemote()
	go p.proxyToLocal()

	return p.localConn.LocalAddr(), err
}

// CloseConn close the localConn
func (p *WGUserSpaceProxy) CloseConn() error {
	p.cancel()
	if p.localConn == nil {
		return nil
	}
	return p.localConn.Close()
}

// Free doing nothing because this implementation of proxy does not have global state
func (p *WGUserSpaceProxy) Free() error {
	return nil
}

// proxyToRemote proxies everything from Wireguard to the RemoteKey peer
// blocks
func (p *WGUserSpaceProxy) proxyToRemote() {

	buf := make([]byte, 1500)
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			n, cm, srcAddr, err := p.packetConn.ReadFrom(buf)
			if err != nil {
				p.log.Errorf("Error reading local wireguard UDP packets:", err)
				continue
			}

			p.log.Debugf("packet (%d bytes) received from (localAddr : %s, readFromSrcAddr : %s)", n, p.localConn.LocalAddr().String(), srcAddr.String())

			p.srcAddr = srcAddr
			if cm != nil {
				p.log.Debugf("control message: %s", cm.Dst)
				p.log.Debugf("control message: %+v", cm)
			}
			if cm.Dst.IsLoopback() {
				addr, err := netip.ParseAddr(cm.Dst.String())
				if err != nil {
					p.log.Errorf("Error parsing packet destination address:", err)
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
func (p *WGUserSpaceProxy) proxyToLocal() {

	buf := make([]byte, 1500)
	for dm := range p.nexRelay.derpRecvCh {
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
		b := dm.src.Raw32()
		pubKey := base64.StdEncoding.EncodeToString(b[:])
		p.log.Debugf("packet (%d bytes) received from (regionId : %s, wgPubKey: %s pubKey : %s)", ncopy, dm.regionID, dm.src.WireGuardGoString(), pubKey)

		if srcIp := p.nexRelay.derpIpMapping.CheckIfKeyExist(pubKey); srcIp != "" {

			dstIp, _,err := net.SplitHostPort(p.srcAddr.String())
			if err != nil {
				p.log.Debugf("Error splitting host port %s : %v", p.srcAddr.String(), err)
				continue
			}

			cm := &ipv4.ControlMessage{TTL: 0,Src: net.ParseIP(srcIp),Dst: net.ParseIP(dstIp),IfIndex: 1}
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

// isLoopbackIP checks if the given IP address is in the range of 127.0.0.0/8 (loopback address range).
func isLoopbackIP(ip net.IP) bool {
	return ip.IsLoopback()
}
