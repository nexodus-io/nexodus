package main

import (
	"fmt"
	"github.com/nexodus-io/nexodus/internal/apex"
	"os"
)

func main() {
	ip := apex.LocalIPv4Address()
	if ip == nil {
		fmt.Fprintln(os.Stderr, "ip not found")
		os.Exit(1)
	}
	fmt.Println(ip.String())
}
