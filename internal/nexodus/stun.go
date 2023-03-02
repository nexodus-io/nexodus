package nexodus

import (
	"fmt"
	"github.com/libp2p/go-reuseport"
	"github.com/pion/stun"
	"go.uber.org/zap"
	"net"
)

const (
	stunServer1 = "stun1.l.google.com:19302"
	stunServer2 = "stun2.l.google.com:19302"
)

func StunRequest(logger *zap.SugaredLogger, stunServer string, srcPort int) (net.UDPAddr, error) {

	logger.Debugf("dialing stun server %s", stunServer)
	conn, err := reuseport.Dial("udp", fmt.Sprintf(":%d", srcPort), stunServer)
	if err != nil {
		logger.Errorf("stun dialing timed out %v", err)
		return net.UDPAddr{}, fmt.Errorf("failed to dial stun server %s: %w", stunServer, err)
	}
	defer func() {
		_ = conn.Close()
	}()

	c, err := stun.NewClient(conn)
	if err != nil {
		logger.Error(err)
		return net.UDPAddr{}, err
	}
	defer func() {
		_ = c.Close()
	}()

	// Building binding request with random transaction id.
	message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	// Sending request to STUN server, waiting for response message.
	result := net.UDPAddr{}
	if err := c.Do(message, func(res stun.Event) {
		if res.Error != nil {
			logger.Error(res.Error)
			return
		}
		// Decoding XOR-MAPPED-ADDRESS attribute from message.
		var xorAddr stun.XORMappedAddress
		if err := xorAddr.GetFrom(res.Message); err != nil {
			return
		}

		result = net.UDPAddr{
			IP:   xorAddr.IP,
			Port: xorAddr.Port,
		}
	}); err != nil {
		logger.Error(err)
		return net.UDPAddr{}, err
	}
	if result.IP.IsUnspecified() {
		return result, fmt.Errorf("no public facing NAT address found for the host")
	}
	logger.Debugf("STUN: your IP:port is: %s:%d", result.IP.String(), result.Port)
	return result, nil
}
