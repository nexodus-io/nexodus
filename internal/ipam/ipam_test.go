package ipam

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"

	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

type IpamTestSuite struct {
	suite.Suite
	logger *zap.SugaredLogger
	ipam   IPAM
	server *http.Server
	wg     sync.WaitGroup
}

func (suite *IpamTestSuite) SetupSuite() {
	suite.server = NewTestIPAMServer()
	suite.logger = zaptest.NewLogger(suite.T()).Sugar()
	suite.ipam = NewIPAM(suite.logger, TestIPAMClientAddr)
	suite.wg = sync.WaitGroup{}
	suite.wg.Add(1)
	listener, err := net.Listen("tcp", "[::1]:9091")
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

func (suite *IpamTestSuite) TestAllocateTunnelIP() {
	ctx := context.Background()
	namespace := uuid.New()
	prefix := "10.20.30.0/24"

	if err := suite.ipam.CreateNamespace(ctx, namespace); err != nil {
		suite.T().Fatal(err)
	}

	if err := suite.ipam.AssignPrefix(ctx, namespace, prefix); err != nil {
		suite.T().Fatal(err)
	}

	// 1. Does not assign invalid IP
	TunnelIP := "notanipaddress"
	_, err := suite.ipam.AssignSpecificTunnelIP(ctx, namespace, prefix, TunnelIP)
	if err == nil {
		suite.T().Fatal("should return an error if ip is invalid")
	}

	// 2. Test valid TunnelIP assigned
	TunnelIP = "10.20.30.1"
	ip, err := suite.ipam.AssignSpecificTunnelIP(ctx, namespace, prefix, TunnelIP)
	if err != nil {
		suite.T().Fatal(err)
	}
	assert.Equal(suite.T(), TunnelIP, ip)

	// 3. Test conflicting TunnelIP assigned from pool
	TunnelIP = "10.20.30.1"
	ip, err = suite.ipam.AssignSpecificTunnelIP(ctx, namespace, prefix, TunnelIP)
	if err != nil {
		suite.T().Fatal(err)
	}
	assert.Equal(suite.T(), "10.20.30.2", ip)

	// 3. Test mismatched TunnelIP assigned from pool
	TunnelIP = "10.20.40.1"
	ip, err = suite.ipam.AssignSpecificTunnelIP(ctx, namespace, prefix, TunnelIP)
	if err != nil {
		suite.T().Fatal(err)
	}
	assert.Equal(suite.T(), "10.20.30.3", ip)
}

func TestIpamTestSuite(t *testing.T) {
	suite.Run(t, new(IpamTestSuite))
}
