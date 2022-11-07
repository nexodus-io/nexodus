package ipam

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"

	"testing"

	"github.com/redhat-et/apex/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type IpamTestSuite struct {
	suite.Suite
	ipam   IPAM
	server *http.Server
	wg     sync.WaitGroup
}

func (suite *IpamTestSuite) SetupSuite() {
	suite.server = util.NewTestIPAMServer()
	suite.ipam = NewIPAM(util.TestIPAMClientAddr)
	suite.wg = sync.WaitGroup{}
	suite.wg.Add(1)
	listener, err := net.Listen("tcp", "[::1]:9090")
	suite.Require().NoError(err)

	go func() {
		defer suite.wg.Done()
		if err := suite.server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
			suite.T().Logf("unexpected error starting ipam server: %s", err)
		}
	}()
}

func (suite *IpamTestSuite) TearDownSuite() {
	suite.server.Close()
	suite.wg.Wait()
}

func (suite *IpamTestSuite) TestAllocateNodeAddress() {
	ctx := context.Background()

	prefix := "10.20.30.0/24"
	if err := suite.ipam.AssignPrefix(ctx, prefix); err != nil {
		suite.T().Fatal(err)
	}

	// 1. Does not assign invalid IP
	nodeAddress := "notanipaddress"
	_, err := suite.ipam.AssignSpecificNodeAddress(ctx, prefix, nodeAddress)
	if err == nil {
		suite.T().Fatal("should return an error if ip is invalid")
	}

	// 2. Test valid NodeAddress assigned
	nodeAddress = "10.20.30.1"
	ip, err := suite.ipam.AssignSpecificNodeAddress(ctx, prefix, nodeAddress)
	if err != nil {
		suite.T().Fatal(err)
	}
	assert.Equal(suite.T(), nodeAddress, ip)

	// 3. Test conflicting NodeAddress assigned from pool
	nodeAddress = "10.20.30.1"
	ip, err = suite.ipam.AssignSpecificNodeAddress(ctx, prefix, nodeAddress)
	if err != nil {
		suite.T().Fatal(err)
	}
	assert.Equal(suite.T(), "10.20.30.2", ip)

	// 3. Test mismatched NodeAddress assigned from pool
	nodeAddress = "10.20.40.1"
	ip, err = suite.ipam.AssignSpecificNodeAddress(ctx, prefix, nodeAddress)
	if err != nil {
		suite.T().Fatal(err)
	}
	assert.Equal(suite.T(), "10.20.30.3", ip)
}

func TestIpamTestSuite(t *testing.T) {
	suite.Run(t, new(IpamTestSuite))
}
