package nexodus

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nexodus-io/nexodus/internal/api/public"
)

func TestRebuildPeerConfig(t *testing.T) {
	nxBase := &Nexodus{
		org: &public.ModelsOrganization{
			Cidr:   "100.64.0.0/10",
			CidrV6: "200::/64",
		},
		nodeReflexiveAddressIPv4: netip.MustParseAddrPort("1.1.1.1:1234"),
	}
	nxRelay := &Nexodus{
		org:                      nxBase.org,
		relay:                    true,
		nodeReflexiveAddressIPv4: netip.MustParseAddrPort("1.1.1.1:1234"),
	}
	nxSymmetricNAT := &Nexodus{
		org:                      nxBase.org,
		symmetricNat:             true,
		nodeReflexiveAddressIPv4: netip.MustParseAddrPort("1.1.1.1:1234"),
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
		// the peering method expected to be chosen based on the local and remote peer parameters
		expectedMethod string
	}{
		{
			// Ensure we choose direct peering when the reflexive IPs are the same
			name:           "direct peering",
			nx:             nxBase,
			peerLocalIP:    "192.168.10.50:5678",
			peerStunIP:     "1.1.1.1:4321",
			expectedMethod: peeringMethodDirectLocal,
		},
		{
			// Ensure we choose reflexive peering when the reflexive IPs are different
			name:           "reflexive peering",
			nx:             nxBase,
			peerLocalIP:    "192.168.10.50:5678",
			peerStunIP:     "2.2.2.2:4321",
			expectedMethod: peeringMethodReflexive,
		},
		{
			// Peer directly with a relay that is behind the same reflexive IP
			name:           "direct peering to relay",
			nx:             nxBase,
			peerLocalIP:    "192.168.10.50:5678",
			peerStunIP:     "1.1.1.1:4321",
			peerIsRelay:    true,
			expectedMethod: peeringMethodRelayPeerDirectLocal,
		},
		{
			// Peer via the reflexive IP of a relay when not behind the same reflexive IP
			name:           "reflexive peering to relay",
			nx:             nxBase,
			peerLocalIP:    "192.168.10.50:5678",
			peerStunIP:     "2.2.2.2:4321",
			peerIsRelay:    true,
			expectedMethod: peeringMethodRelayPeer,
		},
		{
			// We are the relay on the same network as a peer
			name:           "direct peering from relay",
			nx:             nxRelay,
			peerLocalIP:    "192.168.10.50:5678",
			peerStunIP:     "1.1.1.1:4321",
			expectedMethod: peeringMethodRelaySelfDirectLocal,
		},
		{
			// Ensure we choose reflexive peering when the reflexive IPs are different
			name:           "reflexive peering",
			nx:             nxRelay,
			peerLocalIP:    "192.168.10.50:5678",
			peerStunIP:     "2.2.2.2:4321",
			expectedMethod: peeringMethodRelaySelf,
		},
		{
			// Use direct peering when behind the same reflexive IP, even if we are also
			// behind symmetric NAT.
			name:           "direct peering behind symmetric NAT",
			nx:             nxSymmetricNAT,
			peerLocalIP:    "192.168.10.50:5678",
			peerStunIP:     "1.1.1.1:4321",
			expectedMethod: peeringMethodDirectLocal,
		},
		{
			// No peering method available when we are behind symmetric NAT and we
			// have no relay available.
			name:           "no peering method when behind symmetric NAT without a relay",
			nx:             nxSymmetricNAT,
			peerLocalIP:    "192.168.10.50:5678",
			peerStunIP:     "2.2.2.2:4321",
			expectedMethod: "",
		},
		{
			// Use the relay when we are behind symmetric NAT and we have a relay available
			name:           "use relay when we are behind symmetric NAT",
			nx:             nxSymmetricNAT,
			peerLocalIP:    "192.168.10.50:5678",
			peerStunIP:     "2.2.2.2:4321",
			healthyRelay:   true,
			expectedMethod: peeringMethodViaRelay,
		},
		{
			// Use the relay when the peer is behind symmetric NAT, even if we are not
			name:             "use relay when the peer is behind symmetric NAT",
			nx:               nxBase,
			peerLocalIP:      "192.168.10.50:5678",
			peerStunIP:       "2.2.2.2:4321",
			peerSymmetricNAT: true,
			healthyRelay:     true,
			expectedMethod:   peeringMethodViaRelay,
		},
	}

	require := require.New(t)

	for _, tcIter := range testCases {
		tc := tcIter
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := deviceCacheEntry{
				device: public.ModelsDevice{
					Endpoints: []public.ModelsEndpoint{
						{
							Address: tc.peerLocalIP,
							Source:  "local",
						},
						{
							Address: tc.peerStunIP,
							Source:  "stun",
						},
					},
					Relay:        tc.peerIsRelay,
					SymmetricNat: tc.peerSymmetricNAT,
				},
			}
			_, chosenMethod, _ := tc.nx.rebuildPeerConfig(&d, tc.healthyRelay)
			require.Equal(tc.expectedMethod, chosenMethod)
		})
	}
}
