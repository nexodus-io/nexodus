package nexodus

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/api"
	"net"
	"time"

	"github.com/nexodus-io/nexodus/internal/util"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	iterations                         = 1
	interval                           = 800
	timeWait                           = 1200
	PACKETSIZE                         = 64
	ICMP_TYPE_ECHO_REQUEST             = 8
	ICMP_ECHO_REPLY_HEADER_IPV4_OFFSET = 20
	ICMP6_TYPE_ECHO_REQUEST            = 128
)

func (nx *Nexodus) runProbe(peerStatus api.KeepaliveStatus, c chan struct {
	api.KeepaliveStatus
	IsReachable bool
}) {
	latency, err := nx.ping(peerStatus.WgIP)
	if err != nil {
		nx.logger.Debugf("probe error: %v", err)
		c <- struct {
			api.KeepaliveStatus
			IsReachable bool
		}{
			KeepaliveStatus: api.KeepaliveStatus{
				WgIP:     peerStatus.WgIP,
				Hostname: peerStatus.Hostname,
				Latency:  "-",
				Method:   peerStatus.Method,
			},
			IsReachable: false,
		}
	} else {
		c <- struct {
			api.KeepaliveStatus
			IsReachable bool
		}{
			KeepaliveStatus: api.KeepaliveStatus{
				WgIP:     peerStatus.WgIP,
				Hostname: peerStatus.Hostname,
				Latency:  latency,
				Method:   peerStatus.Method,
			},
			IsReachable: true,
		}
	}
}

func cksum(bs []byte) uint16 {
	sum := uint32(0)
	for k := 0; k < len(bs)/2; k++ {
		sum += uint32(bs[k*2]) << 8
		sum += uint32(bs[k*2+1])
	}
	if len(bs)%2 != 0 {
		sum += uint32(bs[len(bs)-1]) << 8
	}
	sum = (sum >> 16) + (sum & 0xffff)
	sum = (sum >> 16) + (sum & 0xffff)
	if sum == 0xffff {
		sum = 0
	}

	return ^uint16(sum)
}

func (nx *Nexodus) doPing(host string, i uint64, waitFor time.Duration) (string, error) {
	if nx.userspaceMode {
		return nx.pingUS(host, i, waitFor)
	} else {
		return nx.pingOS(host, i, waitFor)
	}
}

const (
	protocolICMP     = 1
	protocolIPv6ICMP = 58
)

func (nx *Nexodus) pingUS(host string, i uint64, waitFor time.Duration) (string, error) {
	var networkType string
	var icmpType icmp.Type
	var icmpProto int

	if util.IsIPv6Address(host) {
		networkType = "ping6"
		icmpType = ipv6.ICMPTypeEchoRequest
		icmpProto = protocolIPv6ICMP
	} else {
		networkType = "ping4"
		icmpType = ipv4.ICMPTypeEcho
		icmpProto = protocolICMP
	}
	socket, err := nx.userspaceNet.Dial(networkType, host)
	if err != nil {
		return "", err
	}
	requestPing := icmp.Echo{
		Seq:  int(i),
		Data: []byte("pingity ping"),
	}
	icmpBytes, _ := (&icmp.Message{Type: icmpType, Code: 0, Body: &requestPing}).Marshal(nil)
	err = socket.SetReadDeadline(time.Now().Add(waitFor))
	if err != nil {
		return "", err
	}
	start := time.Now()
	_, err = socket.Write(icmpBytes)
	if err != nil {
		return "", err
	}
	n, err := socket.Read(icmpBytes[:])
	if err != nil {
		return "", err
	}
	replyPacket, err := icmp.ParseMessage(icmpProto, icmpBytes[:n])
	if err != nil {
		return "", err
	}
	replyPing, ok := replyPacket.Body.(*icmp.Echo)
	if !ok {
		return "", fmt.Errorf("invalid reply type: %v", replyPacket)
	}
	if !bytes.Equal(replyPing.Data, requestPing.Data) || replyPing.Seq != requestPing.Seq {
		return "", fmt.Errorf("invalid ping reply: %v", replyPing)
	}
	// calculate latency, round up and return
	latency := time.Since(start)
	roundedLatency := float64(latency) / float64(time.Millisecond)
	nx.logger.Debugf("%d bytes from %v: icmp_seq=%v, time=%v", n, host, i, time.Since(start))

	return fmt.Sprintf("%.2fms", roundedLatency), nil
}

func (nx *Nexodus) pingOS(host string, i uint64, waitFor time.Duration) (string, error) {
	var v6Host bool
	var netname string

	if util.IsIPv6Address(host) {
		v6Host = true
		netname = "ip6:ipv6-icmp"
	} else {
		netname = "ip4:icmp"
	}

	c, err := net.Dial(netname, host)
	if err != nil {
		return "", fmt.Errorf("net.Dial(%v %v) failed: %w", netname, host, err)
	}
	defer c.Close()
	// send echo request
	if err = c.SetDeadline(time.Now().Add(waitFor)); err != nil {
		nx.logger.Debugf("probe error: %v", err)
	}

	msg := make([]byte, PACKETSIZE)
	if v6Host {
		msg[0] = ICMP6_TYPE_ECHO_REQUEST
	} else {
		msg[0] = ICMP_TYPE_ECHO_REQUEST
	}
	msg[1] = 0
	binary.BigEndian.PutUint16(msg[6:], uint16(i))
	binary.BigEndian.PutUint16(msg[4:], uint16(i>>16))
	binary.BigEndian.PutUint16(msg[2:], cksum(msg))
	if _, err := c.Write(msg[:]); err != nil {
		return "", fmt.Errorf("write failed: %w", err)
	}
	// get echo reply
	if err = c.SetDeadline(time.Now().Add(waitFor)); err != nil {
		nx.logger.Debugf("probe error: %v", err)
	}
	rmsg := make([]byte, PACKETSIZE+256)
	start := time.Now()
	amt, err := c.Read(rmsg[:])
	if err != nil {
		return "", fmt.Errorf("read failed: %w", err)
	}

	if !v6Host {
		rmsg = rmsg[ICMP_ECHO_REPLY_HEADER_IPV4_OFFSET:]
	}

	binary.BigEndian.PutUint16(rmsg[2:], 0)
	id := binary.BigEndian.Uint16(rmsg[4:])
	seq := binary.BigEndian.Uint16(rmsg[6:])
	rseq := uint64(id)<<16 + uint64(seq)
	if rseq != i {
		return "", fmt.Errorf("wrong sequence number %v (expected %v)", rseq, i)
	}

	latency := time.Since(start)
	roundedLatency := float64(latency) / float64(time.Millisecond)
	nx.logger.Debugf("ping probe results: %d bytes from %v: icmp_seq=%v, time=%.2fms", amt, host, i, roundedLatency)

	return fmt.Sprintf("%.2fms", roundedLatency), nil
}

func (nx *Nexodus) ping(host string) (string, error) {
	intervalDuration := time.Duration(interval)
	waitFor := time.Duration(timeWait) * time.Millisecond

	var lastResult string

	for i := uint64(0); i <= iterations; i++ {
		result, err := nx.doPing(host, i+1, waitFor)
		if err != nil {
			return "", fmt.Errorf("ping failed: %w", err)
		}
		lastResult = result
		time.Sleep(time.Millisecond * intervalDuration)
	}

	return lastResult, nil
}
