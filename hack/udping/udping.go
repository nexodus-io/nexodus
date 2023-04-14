package main

import (
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	// Parse command line arguments
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <ip> <port>\n", os.Args[0])
		os.Exit(1)
	}
	ip := os.Args[1]
	port := os.Args[2]

	// Create UDP address for destination
	dstAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(ip, port))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error resolving destination address:", err)
		os.Exit(1)
	}

	// Create UDP address for source
	srcAddr := &net.UDPAddr{}

	// Create UDP connection
	conn, err := net.ListenUDP("udp", srcAddr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error creating UDP connection:", err)
		os.Exit(1)
	}
	defer conn.Close()

	// Send UDP packet with payload "ping" to destination
	_, err = conn.WriteToUDP([]byte("ping"), dstAddr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error sending UDP packet:", err)
		os.Exit(1)
	}

	// Wait for UDP packet response
	buffer := make([]byte, 1024)
	err = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error setting read deadline:", err)
	}
	fmt.Printf("Waiting for pong response from %s for 5 seconds...\n", dstAddr.String())
	n, addr, err := conn.ReadFromUDP(buffer)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error receiving UDP packet:", err)
		os.Exit(1)
	}

	// Print the response
	fmt.Printf("Received %d bytes from %s: %s\n", n, addr.String(), string(buffer[:n]))
}
