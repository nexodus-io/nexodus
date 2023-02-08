package main

import (
	"fmt"
	"net"
	"os"
)

func main() {

	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "host argument missing")
		os.Exit(1)
	}
	host := os.Args[1]
	addrs, err := net.LookupHost(host)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %+v\n", err)
		os.Exit(2)
	}

	for _, addr := range addrs {
		fmt.Println(addr)
		return
	}
}
