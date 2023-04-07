//go:build linux

package nexodus

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/pion/stun"
	"go.uber.org/zap"
	"golang.org/x/net/bpf"
	"golang.org/x/net/ipv4"
)

var (
	stunTimeout = 5
)

type udpHeader struct {
	srcPort  uint16
	dstPort  uint16
	length   uint16
	checksum uint16
}

type stunResponse struct {
	xorAddr    *stun.XORMappedAddress
	otherAddr  *stun.OtherAddress
	mappedAddr *stun.MappedAddress
	software   *stun.Software
}

type stunSession struct {
	conn        *ipv4.PacketConn
	innerConn   net.PacketConn
	LocalAddr   net.Addr
	LocalPort   uint16
	RemoteAddr  *net.UDPAddr
	OtherAddr   *net.UDPAddr
	messageChan chan *stun.Message
}

func stunRequest(logger *zap.SugaredLogger, stunSvr string, srcPort int) (netip.AddrPort, error) {
	LocalListenPort := uint16(srcPort)

	conn, err := stunConnect(logger, LocalListenPort, stunSvr)
	defer conn.stunClose()
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("failed to stunConnect to the STUN server: %w", err)
	}

	request := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	responseData, err := conn.stunTransact(logger, request, conn.RemoteAddr)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("transaction error: %w", err)
	}

	response := stunMsgParse(logger, *responseData)
	xorBinding, err := netip.ParseAddrPort(response.xorAddr.String())
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("failed to parse a valid address:port binding from the bpf stun response: %w", err)
	}
	logger.Debugf("reflexive binding is: %s\n", xorBinding.String())

	return xorBinding, nil
}

func (c *stunSession) stunTransact(logger *zap.SugaredLogger, msg *stun.Message, addr net.Addr) (*stun.Message, error) {
	_ = msg.NewTransactionID()
	logger.Debugf("Send to %v: (%v bytes)\n", addr, msg.Length)
	sendUdp := &udpHeader{
		srcPort:  c.LocalPort,
		dstPort:  uint16(c.RemoteAddr.Port),
		length:   uint16(8 + len(msg.Raw)),
		checksum: 0,
	}

	buf := make([]byte, 8)
	binary.BigEndian.PutUint16(buf[0:], sendUdp.srcPort)
	binary.BigEndian.PutUint16(buf[2:], sendUdp.dstPort)
	binary.BigEndian.PutUint16(buf[4:], sendUdp.length)
	binary.BigEndian.PutUint16(buf[6:], sendUdp.checksum)

	if _, err := c.conn.WriteTo(append(buf, msg.Raw...), nil, addr); err != nil {
		return nil, err
	}
	// wait for response
	select {
	case m, ok := <-c.messageChan:
		if !ok {
			return nil, fmt.Errorf("error reading STUN response")
		}
		return m, nil
	case <-time.After(time.Duration(stunTimeout) * time.Second):
		logger.Debugf("bpf STUN request timed out")
		return nil, fmt.Errorf("timed out waiting for stun response")
	}
}

// stunMsgParse parse the STUN response and return them in a stunResponse struct
func stunMsgParse(logger *zap.SugaredLogger, msg stun.Message) stunResponse {
	res := stunResponse{}
	res.mappedAddr = &stun.MappedAddress{}
	res.xorAddr = &stun.XORMappedAddress{}
	res.otherAddr = &stun.OtherAddress{}

	if res.xorAddr.GetFrom(&msg) != nil {
		res.xorAddr = nil
	}
	if res.otherAddr.GetFrom(&msg) != nil {
		res.otherAddr = nil
	}
	if res.mappedAddr.GetFrom(&msg) != nil {
		res.mappedAddr = nil
	}
	if res.software.GetFrom(&msg) != nil {
		res.software = nil
	}

	return res
}

func stunConnect(logger *zap.SugaredLogger, port uint16, addrStr string) (*stunSession, error) {
	addr, err := net.ResolveUDPAddr("udp4", addrStr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve a UDP address: %w ", err)
	}

	conn, err := net.ListenPacket("ip4:udp", "0.0.0.0")
	if err != nil {
		return nil, fmt.Errorf("stun failed to listen on ipv4: %w", err)
	}

	bpfFilter, err := stunBpfFilter(port)
	if err != nil {
		return nil, err
	}
	p := ipv4.NewPacketConn(conn)
	err = p.SetBPF(bpfFilter)
	if err != nil {
		return nil, fmt.Errorf("bpf filter attach error: %w", err)
	}

	mChan := stunListen(logger, p)

	return &stunSession{
		conn:        p,
		innerConn:   conn,
		LocalAddr:   p.LocalAddr(),
		LocalPort:   port,
		RemoteAddr:  addr,
		messageChan: mChan,
	}, nil

}

func stunListen(logger *zap.SugaredLogger, conn *ipv4.PacketConn) (messages chan *stun.Message) {
	messages = make(chan *stun.Message)
	go func() {
		for {
			buf := make([]byte, 1500)
			n, _, addr, err := conn.ReadFrom(buf)
			if err != nil {
				close(messages)
				return
			}
			logger.Debugf("Response from %v: (%v bytes)\n", addr, n)
			// cut UDP header, cut postfix
			buf = buf[8:n]

			m := new(stun.Message)
			m.Raw = buf
			err = m.Decode()
			if err != nil {
				logger.Debugf("error decoding STUN msg: %v\n", err)
				close(messages)
				return
			}
			messages <- m
		}
	}()
	return
}

func stunBpfFilter(port uint16) ([]bpf.RawInstruction, error) {
	var (
		ipOff              uint32 = 0
		udpOff                    = ipOff + 5*4
		payloadOff                = udpOff + 2*4
		stunMagicCookieOff        = payloadOff + 4
		stunMagicCookie    uint32 = 0x2112A442
	)

	r, err := bpf.Assemble([]bpf.Instruction{
		bpf.LoadAbsolute{
			Off:  udpOff + 2,
			Size: 2,
		},
		bpf.JumpIf{
			Cond:      bpf.JumpEqual,
			Val:       uint32(port),
			SkipFalse: 3,
		},
		bpf.LoadAbsolute{
			Off:  stunMagicCookieOff,
			Size: 4,
		},
		bpf.JumpIf{
			Cond:      bpf.JumpEqual,
			Val:       stunMagicCookie,
			SkipFalse: 1,
		},
		bpf.RetConstant{
			Val: 0xffff,
		},
		bpf.RetConstant{
			Val: 0,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("stun bpf filter failed: %v", err)
	}

	return r, nil
}

func (c *stunSession) stunClose() error {
	return c.conn.Close()
}
