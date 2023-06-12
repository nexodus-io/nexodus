package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/nexodus-io/nexodus/internal/signalbus"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/database"
	"github.com/nexodus-io/nexodus/internal/fflags"
	"github.com/nexodus-io/nexodus/internal/ipam"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

const (
	TestUserID     = "f606de8d-092d-4606-b981-80ce9f5a3b2a"
	TestUser2ID    = "3381dcaf-f61e-4671-8787-3e53490894ae"
	ipamClientAddr = "http://localhost:49090"
)

type HandlerTestSuite struct {
	suite.Suite
	logger             *zap.SugaredLogger
	ipam               *http.Server
	wg                 *sync.WaitGroup
	api                *API
	testOrganizationID uuid.UUID
	testUser2OrgID     uuid.UUID
}

func (suite *HandlerTestSuite) SetupSuite() {
	db, err := database.NewTestDatabase()
	if err != nil {
		suite.T().Fatal(err)
	}
	suite.logger = zaptest.NewLogger(suite.T()).Sugar()
	suite.ipam = ipam.NewTestIPAMServer()
	suite.wg = &sync.WaitGroup{}
	suite.wg.Add(1)

	listener, err := net.Listen("tcp", "[::1]:49090")
	suite.Require().NoError(err)

	go func() {
		defer suite.wg.Done()
		if err := suite.ipam.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
			suite.T().Logf("unexpected error starting ipam server: %s", err)
		}
	}()

	ipamClient := ipam.NewIPAM(suite.logger, ipamClientAddr)

	fflags := fflags.NewFFlags(suite.logger)
	store := inmem.New()
	suite.api, err = NewAPI(context.Background(), suite.logger, db, ipamClient, fflags, store, signalbus.NewSignalBus())
	if err != nil {
		suite.T().Fatal(err)
	}
}

func (suite *HandlerTestSuite) BeforeTest(_, _ string) {
	suite.api.db.Exec("DELETE FROM users")
	suite.api.db.Exec("DELETE FROM organizations")
	suite.api.db.Exec("DELETE FROM user_organizations")
	suite.api.db.Exec("DELETE FROM devices")
	var err error
	suite.testOrganizationID, err = suite.api.createUserIfNotExists(context.Background(), TestUserID, "testuser")
	suite.Require().NoError(err)
	suite.testUser2OrgID, err = suite.api.createUserIfNotExists(context.Background(), TestUser2ID, "testuser2")
	suite.Require().NoError(err)
}

func (suite *HandlerTestSuite) ServeRequest(method, path string, uri string, handler func(*gin.Context), body io.Reader) (*http.Request, *httptest.ResponseRecorder, error) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(gin.AuthUserKey, TestUserID)
		c.Next()
	})
	r.Use(suite.api.CreateUserIfNotExists())
	r.Any(path, handler)
	req, err := http.NewRequest(method, uri, body)
	if err != nil {
		return req, httptest.NewRecorder(), err
	}
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	return req, res, nil
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func TestQuerySort(t *testing.T) {
	q := Query{Sort: `["name","DESC"]`}
	expected := "name DESC"
	actual, err := q.GetSort()
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestQueryRange(t *testing.T) {
	q := Query{Range: `[ 0, 24 ]`}
	expectedPageSize := 25
	expectedOffset := 0
	actualPageSize, actualOffset, err := q.GetRange()
	assert.NoError(t, err)
	assert.Equal(t, expectedPageSize, actualPageSize)
	assert.Equal(t, expectedOffset, actualOffset)
}

func TestQueryFilter(t *testing.T) {
	q := Query{Filter: `{ "title": "bar" }`}
	expected := map[string]interface{}{"title": "bar"}
	actual, err := q.GetFilter()
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func (suite *HandlerTestSuite) jsonMarshal(v any) []byte {
	bytes, err := json.Marshal(v)
	suite.Require().NoError(err)
	return bytes
}
