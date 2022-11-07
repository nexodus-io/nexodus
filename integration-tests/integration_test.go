//go:build integration
// +build integration

package integration_tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ApexIntegrationSuite struct {
	suite.Suite
	pool *dockertest.Pool
}

func (suite *ApexIntegrationSuite) SetupSuite() {
	var err error
	suite.pool, err = dockertest.NewPool("")
	require.NoError(suite.T(), err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err = backoff.Retry(healthcheck, backoff.WithContext(backoff.NewExponentialBackOff(), ctx))
	require.NoError(suite.T(), err)
}

func TestApexIntegrationSuite(t *testing.T) {
	suite.Run(t, new(ApexIntegrationSuite))
}

func (suite *ApexIntegrationSuite) TestBasicConnectivity() {
	assert := suite.Assert()
	require := suite.Require()

	token, err := GetToken("admin", "floofykittens")
	require.NoError(err)

	node1 := suite.CreateNode("node1",
		[]string{
			fmt.Sprintf("--with-token=%s", token),
			"http://host.docker.internal:8080",
		},
	)
	defer node1.Close()

	node2 := suite.CreateNode("node2",
		[]string{
			fmt.Sprintf("--with-token=%s", token),
			"http://host.docker.internal:8080",
		},
	)
	defer node2.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	node1IP, err := getWg0IP(ctx, node1)
	require.NoError(err)
	node2IP, err := getWg0IP(ctx, node2)
	require.NoError(err)

	suite.T().Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, node2IP)
	assert.NoError(err)

	suite.T().Logf("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, node1IP)
	assert.NoError(err)
}
