package dnsserver

import (
	"context"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	_ "github.com/coredns/coredns/plugin/bind"
	_ "github.com/coredns/coredns/plugin/forward"
	_ "github.com/coredns/coredns/plugin/hosts"
)

func TestStart(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server, err := Start(ctx, nil, `
			.:0 {
					bind 127.0.0.1
					hosts {
							10.0.0.1 example.org
							ttl 60
							fallthrough
					}
					forward . 8.8.8.8 8.8.4.4 
			}
		`)
	require.NoError(err)

	listenPort, _, err := server.Ports()
	require.NoError(err)

	d := &dns.Client{
		Timeout: 5 * time.Second,
	}

	// Verify resolving a simple custom hosts works...
	m := &dns.Msg{}
	m.SetQuestion("example.org.", dns.TypeA)
	resp, _, err := d.Exchange(m, listenPort.String())
	require.NoError(err)
	require.Equal(dns.RcodeSuccess, resp.Rcode)
	require.Equal(1, len(resp.Answer))
	require.Equal("example.org.\t60\tIN\tA\t10.0.0.1", resp.Answer[0].String())

	// Verify resolving public dns name works...
	m = &dns.Msg{}
	m.SetQuestion("hiramchirino.com.", dns.TypeA)
	resp, _, err = d.Exchange(m, listenPort.String())
	require.NoError(err)
	require.Equal(dns.RcodeSuccess, resp.Rcode)
	require.Equal(1, len(resp.Answer))
	require.Contains(resp.Answer[0].String(), "216.24.57.1")

}
