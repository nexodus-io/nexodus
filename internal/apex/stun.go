package apex

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/pion/stun"
	"go.uber.org/zap"
)

const (
	stunServer1 = "stun1.l.google.com:19302"
	stunServer2 = "stun2.l.google.com:19302"
)

// IsSymmetricNAT attempts to infer if the node is behind a symmetric
// nat device by querying two STUN servers. If the requests return
// different ports, then it is likely the node is behind a symmetric nat.
func IsSymmetricNAT(logger *zap.SugaredLogger) (bool, error) {
	// get a random port to source the request from since the wg port may be bound
	srcPort, err := getWgListenPort()
	if err != nil {
		return false, fmt.Errorf("failed to queury the STUN server %s: %w", stunServer1, err)
	}
	firstStun, err := StunRequest(logger, stunServer1, srcPort)
	if err != nil {
		return false, fmt.Errorf("failed to queury the STUN server %s", stunServer1)
	}
	secondStun, err := StunRequest(logger, stunServer2, srcPort)
	if err != nil {
		return false, fmt.Errorf("failed to queury the STUN server %s", stunServer1)
	}
	if firstStun != secondStun {
		return true, nil
	}

	return false, nil
}

// StunRequest initiate a connection to a STUN server sourced from the wg src port
func StunRequest(logger *zap.SugaredLogger, stunServer string, srcPort int) (string, error) {
	lAddr := &net.UDPAddr{
		Port: srcPort,
	}
	d := &net.Dialer{
		Timeout:   3 * time.Second,
		LocalAddr: lAddr,
	}
	logger.Debugf("dialing stun server %s", stunServer)
	conn, err := d.Dial("udp4", stunServer)
	if err != nil {
		logger.Errorf("stun dialing timed out %v", err)
		return "", fmt.Errorf("failed to dial stun server %s: %w", stunServer, err)
	}

	stunResults, err := stunDialer(logger, &conn)
	if err != nil {
		return "", fmt.Errorf("stun dialing timed out: %w", err)
	}
	return stunResults, nil
}

func stunDialer(logger *zap.SugaredLogger, conn *net.Conn) (string, error) {
	c, err := stun.NewClient(*conn)
	if err != nil {
		logger.Errorf("Failed to open a stun socket %v", err)
	}
	var xorAddr stun.XORMappedAddress
	if err = c.Do(stun.MustBuild(stun.TransactionID, stun.BindingRequest), func(res stun.Event) {
		if res.Error != nil {
			logger.Errorf("stun request error %v", res.Error)
			return
		}
		if err := xorAddr.GetFrom(res.Message); err != nil {
			logger.Errorf("stun request error %v", res)
			if err := c.Close(); err != nil {
				logger.Errorf("stun request error %v", res)
				return
			}
			return
		}
		logger.Debugf("Stun address and port is: %s:%d", xorAddr.IP, xorAddr.Port)

	}); err != nil {
		return "", err
	}
	if err := c.Close(); err != nil {
		return "", err
	}
	stunAddress := net.JoinHostPort(xorAddr.IP.String(), strconv.Itoa(xorAddr.Port))
	if err != nil {
		return "", err
	}

	return stunAddress, nil
}

// GetPubIPv4 retrieves current global IP address using STUN
func GetPubIPv4(logger *zap.SugaredLogger) (string, error) {
	// Creating a "connection" to STUN server.
	c, err := stun.Dial("udp4", "stun.l.google.com:19302")
	if err != nil {
		logger.Error(err)
		return "", err
	}

	// Building binding request with random transaction id.
	message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	var ourIP string
	// Sending request to STUN server, waiting for response message.
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
		logger.Debug("STUN: your IP is: ", xorAddr.IP)
		ourIP = xorAddr.IP.String()
	}); err != nil {
		logger.Error(err)
		return "", err
	}

	return ourIP, nil
}
