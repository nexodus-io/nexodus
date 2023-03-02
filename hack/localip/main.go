package main

import (
	"fmt"
	"os"

	"github.com/nexodus-io/nexodus/internal/nexodus"
)

func main() {
	ip := nexodus.LocalIPv4Address()
	if ip == nil {
		fmt.Fprintln(os.Stderr, "ip not found")
		os.Exit(1)
	}
	fmt.Println(ip.String())
}
