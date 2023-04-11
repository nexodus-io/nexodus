package nexodus

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"go.uber.org/zap"
)

type dialer struct {
	logger zap.SugaredLogger
	dial   func(string, string) (net.Conn, error)
}

const (
	iterations                         = 1
	interval                           = 800
	timeWait                           = 1200
	PACKETSIZE                         = 64
	ICMP_TYPE_ECHO_REQUEST             = 8
	ICMP_ECHO_REPLY_HEADER_IPV4_OFFSET = 20
)

type KeepaliveStatus struct {
	WgIP        string `json:"wg_ip"`
	IsReachable bool   `json:"is_reachable"`
	Hostname    string `json:"hostname"`
}

func runProbe(peerStatus KeepaliveStatus, c chan struct {
	KeepaliveStatus
	IsReachable bool
}) {
	err := ping(peerStatus.WgIP)
	if err != nil {
		// peer is not replying
		c <- struct {
			KeepaliveStatus
			IsReachable bool
		}{peerStatus, false}
	} else {
		// peer is replying
		c <- struct {
			KeepaliveStatus
			IsReachable bool
		}{peerStatus, true}
	}
}

func newPinger() *dialer {
	return &dialer{
		dial: net.Dial,
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

func (p *dialer) ping(host string, i uint64, waitFor time.Duration) (string, error) {
	netname := "ip4:icmp"
	c, err := p.dial(netname, host)
	if err != nil {
		return "", fmt.Errorf("net.Dial(%v %v) failed: %w", netname, host, err)
	}
	defer c.Close()
	// send echo request
	if err = c.SetDeadline(time.Now().Add(waitFor)); err != nil {
		p.logger.Debugf("probe error: %v", err)
	}

	msg := make([]byte, PACKETSIZE)
	msg[0] = ICMP_TYPE_ECHO_REQUEST
	msg[1] = 0
	binary.BigEndian.PutUint16(msg[6:], uint16(i))
	binary.BigEndian.PutUint16(msg[4:], uint16(i>>16))
	binary.BigEndian.PutUint16(msg[2:], cksum(msg))
	if _, err := c.Write(msg[:]); err != nil {
		return "", fmt.Errorf("write failed: %w", err)
	}
	// get echo reply
	if err = c.SetDeadline(time.Now().Add(waitFor)); err != nil {
		p.logger.Debugf("probe error: %v", err)
	}
	rmsg := make([]byte, PACKETSIZE+256)
	before := time.Now()
	amt, err := c.Read(rmsg[:])
	if err != nil {
		return "", fmt.Errorf("read failed: %w", err)
	}
	latency := time.Since(before)

	rmsg = rmsg[ICMP_ECHO_REPLY_HEADER_IPV4_OFFSET:]
	cks := binary.BigEndian.Uint16(rmsg[2:])
	binary.BigEndian.PutUint16(rmsg[2:], 0)
	if cks != cksum(rmsg) {
		return "", fmt.Errorf("bad ICMP checksum: %v (expected %v)", cks, cksum(rmsg))
	}
	id := binary.BigEndian.Uint16(rmsg[4:])
	seq := binary.BigEndian.Uint16(rmsg[6:])
	rseq := uint64(id)<<16 + uint64(seq)
	if rseq != i {
		return "", fmt.Errorf("wrong sequence number %v (expected %v)", rseq, i)
	}

	return fmt.Sprintf("%d bytes from %v: icmp_seq=%v, time=%v", amt, host, i, latency), nil
}

func ping(host string) error {
	if PACKETSIZE < 8 {
		return fmt.Errorf("packet size too small (must be >= 8): %v", PACKETSIZE)
	}
	interval := time.Duration(interval)
	p := newPinger()
	waitFor := time.Duration(timeWait) * time.Millisecond
	for i := uint64(0); i <= iterations; i++ {
		_, err := p.ping(host, i+1, waitFor)
		if err != nil {
			return fmt.Errorf("ping failed: %w", err)
		}
		// TODO: this is probably irrelevant on one iteration
		time.Sleep(time.Millisecond * interval)
	}
	return nil
}
