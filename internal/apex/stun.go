package apex

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/pion/stun"
	log "github.com/sirupsen/logrus"
)

const (
	stunServer1 = "stun1.l.google.com:19302"
	stunServer2 = "stun2.l.google.com:19302"
)

// IsSymmetricNAT attempts to infer if the node is behind a symmetric
// nat device by querying two STUN servers. If the requests return
// different ports, then it is likely the node is behind a symmetric nat.
func IsSymmetricNAT() (bool, error) {
	// get a random port to source the request from since the wg port may be bound
	srcPort := getWgListenPort()
	firstStun, err := StunRequest(stunServer1, srcPort)
	if err != nil {
		return false, fmt.Errorf("failed to queury the STUN server %s", stunServer1)
	}
	secondStun, err := StunRequest(stunServer2, srcPort)
	if err != nil {
		return false, fmt.Errorf("failed to queury the STUN server %s", stunServer1)
	}
	if firstStun != secondStun {
		return true, nil
	}

	return false, nil
}

// StunRequest initiate a connection to a STUN server sourced from the wg src port
func StunRequest(stunServer string, srcPort int) (string, error) {
	lAddr := &net.UDPAddr{
		Port: srcPort,
	}
	d := &net.Dialer{
		Timeout:   3 * time.Second,
		LocalAddr: lAddr,
	}
	log.Debugf("dialing stun server %s", stunServer)
	conn, err := d.Dial("udp4", stunServer)
	if err != nil {
		log.Errorf("stun dialing timed out %v", err)
		return "", fmt.Errorf("failed to dial stun server %s: %v", stunServer, err)
	}

	stunResults, err := stunDialer(&conn)
	if err != nil {
		return "", fmt.Errorf("stun dialing timed out %v", err)
	}
	return stunResults, nil
}

func stunDialer(conn *net.Conn) (string, error) {
	c, err := stun.NewClient(*conn)
	if err != nil {
		log.Errorf("Failed to open a stun socket %v", err)
	}
	var xorAddr stun.XORMappedAddress
	if err = c.Do(stun.MustBuild(stun.TransactionID, stun.BindingRequest), func(res stun.Event) {
		if res.Error != nil {
			log.Errorf("stun request error %v", res.Error)
			return
		}
		if err := xorAddr.GetFrom(res.Message); err != nil {
			log.Errorf("stun request error %v", res)
			if err := c.Close(); err != nil {
				log.Errorf("stun request error %v", res)
				return
			}
			return
		}
		log.Debugf("Stun address and port is: %s:%d", xorAddr.IP, xorAddr.Port)

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
