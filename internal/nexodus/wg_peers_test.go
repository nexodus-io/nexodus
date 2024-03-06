package nexodus

import (
	"github.com/nexodus-io/nexodus/internal/client"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRebuildPeerConfig(t *testing.T) {
	zLogger, _ := zap.NewDevelopment()
	testLogger := zLogger.Sugar()
	nxBase := &Nexodus{
		vpc: &client.ModelsVPC{
			Ipv4Cidr: client.PtrString("100.64.0.0/10"),
			Ipv6Cidr: client.PtrString("200::/64"),
		},
		nodeReflexiveAddressIPv4: netip.MustParseAddrPort("1.1.1.1:1234"),
		logger:                   testLogger,
	}
	nxRelay := &Nexodus{
		vpc:                      nxBase.vpc,
		relay:                    true,
		nodeReflexiveAddressIPv4: netip.MustParseAddrPort("1.1.1.1:1234"),
		logger:                   testLogger,
	}
	nxSymmetricNAT := &Nexodus{
		vpc:                      nxBase.vpc,
		symmetricNat:             true,
		nodeReflexiveAddressIPv4: netip.MustParseAddrPort("1.1.1.1:1234"),
		logger:                   testLogger,
	}
	nxDerpRelay := &Nexodus{
		vpc:                      nxBase.vpc,
		symmetricNat:             true,
		nodeReflexiveAddressIPv4: netip.MustParseAddrPort("1.1.1.1:1234"),
		logger:                   testLogger,
		nexRelay: nexRelay{
			derpIpMapping: NewDerpIpMapping(),
		},
	}

	testCases := []struct {
		// descriptive name of the test case
		name string
		// the parameters set in the Nexodus object reflect the parameters for the local node
		nx               *Nexodus
		peerLocalIP      string
		peerStunIP       string
		peerIsRelay      bool
		peerSymmetricNAT bool
		// we have a healthy relay available
		healthyRelay bool
		//is it derp relay available
		relay bool
		// the peering method expected to be chosen based on the local and remote peer parameters
		expectedMethod string
		// the second choice peering method
		secondMethod string
		// the third choice peering method
		thirdMethod string
	}{
		{
			// Ensure we choose direct peering when the reflexive IPs are the same
			name:           "direct peering",
			nx:             nxBase,
			peerLocalIP:    "192.168.10.50:5678",
			peerStunIP:     "1.1.1.1:4321",
			expectedMethod: peeringMethodDirectLocal,
			secondMethod:   peeringMethodReflexive,
			thirdMethod:    peeringMethodDirectLocal,
		},
		{
			// Ensure we choose direct peering when the reflexive IPs are the same and fall back to a relay
			name:           "direct peering",
			nx:             nxBase,
			peerLocalIP:    "192.168.10.50:5678",
			peerStunIP:     "1.1.1.1:4321",
			healthyRelay:   true,
			relay:          true,
			expectedMethod: peeringMethodDirectLocal,
			secondMethod:   peeringMethodReflexive,
			thirdMethod:    peeringMethodViaRelay,
		},
		{
			// Ensure we choose reflexive peering when the reflexive IPs are different
			name:           "reflexive peering",
			nx:             nxBase,
			peerLocalIP:    "192.168.10.50:5678",
			peerStunIP:     "2.2.2.2:4321",
			expectedMethod: peeringMethodReflexive,
			secondMethod:   peeringMethodReflexive, // our only choice
			thirdMethod:    peeringMethodReflexive, // our only choice
		},
		{
			// Ensure we choose reflexive peering when the reflexive IPs are different
			name:           "reflexive peering",
			nx:             nxBase,
			peerLocalIP:    "192.168.10.50:5678",
			peerStunIP:     "2.2.2.2:4321",
			healthyRelay:   true,
			relay:          true,
			expectedMethod: peeringMethodReflexive,
			secondMethod:   peeringMethodViaRelay,
			thirdMethod:    peeringMethodViaRelay, // stay with a healthy relay
		},
		{
			// Peer directly with a relay that is behind the same reflexive IP
			name:           "direct peering to relay",
			nx:             nxBase,
			peerLocalIP:    "192.168.10.50:5678",
			peerStunIP:     "1.1.1.1:4321",
			peerIsRelay:    true,
			relay:          true,
			expectedMethod: peeringMethodRelayPeerDirectLocal,
			secondMethod:   peeringMethodRelayPeer,
			thirdMethod:    peeringMethodRelayPeerDirectLocal,
		},
		{
			// Peer via the reflexive IP of a relay when not behind the same reflexive IP
			name:           "reflexive peering to relay",
			nx:             nxBase,
			peerLocalIP:    "192.168.10.50:5678",
			peerStunIP:     "2.2.2.2:4321",
			peerIsRelay:    true,
			relay:          true,
			expectedMethod: peeringMethodRelayPeer,
			secondMethod:   peeringMethodRelayPeer, // our only choice
			thirdMethod:    peeringMethodRelayPeer, // our only choice
		},
		{
			// We are the relay on the same network as a peer
			name:           "direct peering from relay",
			nx:             nxRelay,
			peerLocalIP:    "192.168.10.50:5678",
			peerStunIP:     "1.1.1.1:4321",
			expectedMethod: peeringMethodRelaySelfDirectLocal,
			secondMethod:   peeringMethodRelaySelf,
			thirdMethod:    peeringMethodRelaySelfDirectLocal,
		},
		{
			// Ensure we choose reflexive peering when the reflexive IPs are different
			name:           "reflexive peering",
			nx:             nxRelay,
			peerLocalIP:    "192.168.10.50:5678",
			peerStunIP:     "2.2.2.2:4321",
			expectedMethod: peeringMethodRelaySelf,
			secondMethod:   peeringMethodRelaySelf, // our only choice
			thirdMethod:    peeringMethodRelaySelf, // our only choice
		},
		{
			// Use direct peering when behind the same reflexive IP, even if we are also
			// behind symmetric NAT.
			name:           "direct peering behind symmetric NAT",
			nx:             nxSymmetricNAT,
			peerLocalIP:    "192.168.10.50:5678",
			peerStunIP:     "1.1.1.1:4321",
			expectedMethod: peeringMethodDirectLocal,
			secondMethod:   peeringMethodDirectLocal, // our only choice without a relay
			thirdMethod:    peeringMethodDirectLocal, // our only choice without a relay
		},
		{
			// No peering method available when we are behind symmetric NAT and we
			// have no legacy relay available, but public derp relay available.
			name:           "no peering method when behind symmetric NAT without a relay",
			nx:             nxDerpRelay,
			peerLocalIP:    "192.168.10.50:5678",
			peerStunIP:     "2.2.2.2:4321",
			expectedMethod: peeringMethodViaDerpRelay,
			secondMethod:   peeringMethodViaDerpRelay,
			thirdMethod:    peeringMethodViaDerpRelay,
		},
		{
			// Use the relay when we are behind symmetric NAT and we have a relay available
			name:           "use relay when we are behind symmetric NAT",
			nx:             nxSymmetricNAT,
			peerLocalIP:    "192.168.10.50:5678",
			peerStunIP:     "2.2.2.2:4321",
			healthyRelay:   true,
			relay:          true,
			expectedMethod: peeringMethodViaRelay,
			secondMethod:   peeringMethodViaRelay, // our only choice
			thirdMethod:    peeringMethodViaRelay, // our only choice
		},
		{
			// Use the relay when the peer is behind symmetric NAT, even if we are not
			name:             "use relay when the peer is behind symmetric NAT",
			nx:               nxBase,
			peerLocalIP:      "192.168.10.50:5678",
			peerStunIP:       "2.2.2.2:4321",
			peerSymmetricNAT: true,
			healthyRelay:     true,
			relay:            true,
			expectedMethod:   peeringMethodViaRelay,
			secondMethod:     peeringMethodViaRelay, // our only choice
			thirdMethod:      peeringMethodViaRelay, // our only choice
		},
	}

	require := require.New(t)

	for _, tcIter := range testCases {
		tc := tcIter
		t.Run(tc.name, func(t *testing.T) {
			d := deviceCacheEntry{
				device: client.ModelsDevice{
					Endpoints: []client.ModelsEndpoint{
						{
							Address: client.PtrString(tc.peerLocalIP),
							Source:  client.PtrString("local"),
						},
						{
							Address: client.PtrString(tc.peerStunIP),
							Source:  client.PtrString("stun"),
						},
					},
					PublicKey:    client.PtrString("bacon"),
					Relay:        client.PtrBool(tc.peerIsRelay),
					SymmetricNat: client.PtrBool(tc.peerSymmetricNAT),
				},
			}
			tc.nx.peeringReset(&d)

			_, chosenMethod, chosenIndex := tc.nx.rebuildPeerConfig(&d, tc.healthyRelay, tc.relay)
			require.Equal(tc.expectedMethod, chosenMethod)

			now := time.Now()

			// Ensure we stick with the chosen method while the peer is healthy.
			// Set that we peered 15 minutes ago and it was healthy 5 seconds later.
			d.peerHealthy = true
			d.peeringMethod = chosenMethod
			d.peeringMethodIndex = chosenIndex
			d.peeringTime = now.Add(-15 * time.Minute)
			d.peerHealthyTime = now.Add(-15*time.Minute + 5*time.Second)
			_, chosenMethod, _ = tc.nx.rebuildPeerConfig(&d, tc.healthyRelay, tc.relay)
			require.Equal(tc.expectedMethod, chosenMethod)

			// Switch to unhealthy, but since this was previously a successful
			// peer configuration, it should keep trying for 3 minutes. We
			// have 1 minute until the deadline to work again.
			d.peerHealthy = false
			d.peerHealthyTime = now.Add(-1*peeringRestoreTimeout + time.Minute)
			_, chosenMethod, _ = tc.nx.rebuildPeerConfig(&d, tc.healthyRelay, tc.relay)
			require.Equal(tc.expectedMethod, chosenMethod)

			// After 3 minutes, we should switch to the next best method.
			// We were last healthy 3 minutes and 5 seconds ago.
			d.peerHealthyTime = now.Add(-1*peeringRestoreTimeout - 5*time.Second)
			_, chosenMethod, chosenIndex = tc.nx.rebuildPeerConfig(&d, tc.healthyRelay, tc.relay)
			require.Equal(tc.secondMethod, chosenMethod)

			// Another recalculation should switch to the third best method.
			d.peeringMethod = chosenMethod
			d.peeringMethodIndex = chosenIndex
			_, chosenMethod, _ = tc.nx.rebuildPeerConfig(&d, tc.healthyRelay, tc.relay)
			require.Equal(tc.thirdMethod, chosenMethod)
		})
	}
}

func TestBuildPeersConfig(t *testing.T) {
	zLogger, _ := zap.NewDevelopment()
	testLogger := zLogger.Sugar()
	require := require.New(t)

	//
	// The test scenario is encoded in this Nexodus instance.
	//
	// We have 3 devices we are peered with:
	// - directPeerWithAdvertiseCidrs: a peer we can reach directly, and it has a child prefix.
	//		In this case, we should see the child prefix in the peer config.
	//		This child prefix should NOT be present in the relay configuration.
	// - peerViaRelayWithAdvertiseCidrs: a peer we can reach via a relay, and it has a child prefix.
	//		In this case, since we are unable to peer with this device directly,
	//		the child prefix should be reachable via the relay.
	// - theRelay: a relay we can reach directly.
	//
	nx := &Nexodus{
		vpc: &client.ModelsVPC{
			Ipv4Cidr: client.PtrString("100.64.0.0/10"),
			Ipv6Cidr: client.PtrString("200::/64"),
		},
		nodeReflexiveAddressIPv4: netip.MustParseAddrPort("1.1.1.1:1234"),
		logger:                   testLogger,
		deviceCache: map[string]deviceCacheEntry{
			"directPeerWithAdvertiseCidrs": {
				device: client.ModelsDevice{
					Endpoints: []client.ModelsEndpoint{
						{
							Address: client.PtrString("192.168.50.2:5678"),
							Source:  client.PtrString("local"),
						},
						{
							Address: client.PtrString("2.2.2.2:4321"),
							Source:  client.PtrString("stun"),
						},
					},
					PublicKey: client.PtrString("directPeerWithAdvertiseCidrs"),
					AdvertiseCidrs: []string{
						"192.168.50.0/24",
					},
				},
			},
			"peerViaRelayWithAdvertiseCidrs": {
				device: client.ModelsDevice{
					Endpoints: []client.ModelsEndpoint{
						{
							Address: client.PtrString("192.168.40.2:5678"),
							Source:  client.PtrString("local"),
						},
						{
							Address: client.PtrString("2.2.2.2:4321"),
							Source:  client.PtrString("stun"),
						},
					},
					PublicKey:    client.PtrString("peerViaRelayWithAdvertiseCidrs"),
					SymmetricNat: client.PtrBool(true),
					AdvertiseCidrs: []string{
						"192.168.40.0/24",
					},
				},
			},
			"theRelay": {
				device: client.ModelsDevice{
					Endpoints: []client.ModelsEndpoint{
						{
							Address: client.PtrString("192.168.30.5:5678"),
							Source:  client.PtrString("local"),
						},
						{
							Address: client.PtrString("3.3.3.3:4321"),
							Source:  client.PtrString("stun"),
						},
					},
					PublicKey: client.PtrString("theRelay"),
					Relay:     client.PtrBool(true),
				},
			},
		},
	}

	for _, dIter := range nx.deviceCache {
		d := dIter
		nx.peeringReset(&d)
		d.peerHealthy = true
		nx.deviceCache[d.device.GetPublicKey()] = d
	}

	updatedDevices := nx.buildPeersConfig()
	require.Equal(len(updatedDevices), 3)

	// Since one peer is reached via a relay, we should have 2 peers in the wireguard config.
	require.Equal(len(nx.wgConfig.Peers), 2)

	// The child prefix for the peer we can reach directly is in the config for that peer and not the relay.
	require.Contains(nx.wgConfig.Peers["directPeerWithAdvertiseCidrs"].AllowedIPs, "192.168.50.0/24")
	require.NotContains(nx.wgConfig.Peers["theRelay"].AllowedIPs, "192.168.50.0/24")

	// The child prefix for the peer we can reach via the relay is in the config for the relay.
	// We should have no config for the peer itself.
	require.NotContains(nx.wgConfig.Peers, "peerViaRelayWithAdvertiseCidrs")
	require.Contains(nx.wgConfig.Peers["theRelay"].AllowedIPs, "192.168.40.0/24")
}
