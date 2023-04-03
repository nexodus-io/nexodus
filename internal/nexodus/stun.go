package nexodus

import (
	"errors"
	"fmt"
	"net"

	"github.com/libp2p/go-reuseport"
	"github.com/pion/stun"
	"go.uber.org/zap"
)

const (
	stunServer1 = "stun1.l.google.com:19302"
	stunServer2 = "stun2.l.google.com:19302"
)

func stunRequest(logger *zap.SugaredLogger, stunServer string, srcPort int) (net.UDPAddr, error) {

	logger.Debugf("dialing stun server %s", stunServer)
	conn, err := reuseport.Dial("udp4", fmt.Sprintf(":%d", srcPort), stunServer)
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
			if res.Error.Error() == errors.New("transaction is timed out").Error() {
				logger.Debugf("STUN transaction to %s timed out", stunServer)
			} else {
				logger.Debug(res.Error)
			}
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
		return net.UDPAddr{}, err
	}
	if result.IP.IsUnspecified() {
		return result, fmt.Errorf("no public facing NAT address found for the host")
	}
	if result.IP == nil {
		return result, fmt.Errorf("STUN binding request failed, a firewall may be blocking UDP connections to %s", stunServer)
	}
	logger.Debugf("STUN: your IP:port is: %s:%d", result.IP.String(), result.Port)
	return result, nil
}

// symmetricNatDisco determine if the joining node is within a symmetric NAT cone
func symmetricNatDisco(logger *zap.SugaredLogger, wgListenPort int) (bool, string, error) {

	nodeReflexiveAddress := ""
	isSymmetric := false
	// discover the server reflexive address per ICE RFC8445
	stunAddr, err := stunRequest(logger, stunServer1, wgListenPort)
	if err != nil {
		return isSymmetric, nodeReflexiveAddress, err
	} else {
		nodeReflexiveAddress = stunAddr.IP.String()
	}

	stunAddr2, err := stunRequest(logger, stunServer2, wgListenPort)
	if err != nil {
		return false, "", err
	} else {
		isSymmetric = stunAddr.String() != stunAddr2.String()
	}

	return isSymmetric, nodeReflexiveAddress, nil
}

// getReflexiveAddress returns the reflexive address
func getReflexiveAddress(logger *zap.SugaredLogger, wgListenPort int) (string, error) {

	nodeReflexiveAddress := ""
	stunAddr, err := stunRequest(logger, stunServer1, wgListenPort)
	if err != nil {
		return nodeReflexiveAddress, err
	} else {
		nodeReflexiveAddress = stunAddr.IP.String()
	}

	return nodeReflexiveAddress, nil
}
