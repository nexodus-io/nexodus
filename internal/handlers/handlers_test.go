package handlers

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/database"
	"github.com/redhat-et/apex/internal/fflags"
	"github.com/redhat-et/apex/internal/ipam"
	"github.com/redhat-et/apex/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

const (
	TestUserID = "f606de8d-092d-4606-b981-80ce9f5a3b2a"
)

type HandlerTestSuite struct {
	suite.Suite
	logger             *zap.SugaredLogger
	ipam               *http.Server
	wg                 *sync.WaitGroup
	api                *API
	testOrganizationID uuid.UUID
}

func (suite *HandlerTestSuite) SetupSuite() {
	db, err := database.NewTestDatabase()
	if err != nil {
		suite.T().Fatal(err)
	}
	suite.logger = zaptest.NewLogger(suite.T()).Sugar()
	suite.ipam = util.NewTestIPAMServer()
	suite.wg = &sync.WaitGroup{}
	suite.wg.Add(1)

	listener, err := net.Listen("tcp", "[::1]:9090")
	suite.Require().NoError(err)

	go func() {
		defer suite.wg.Done()
		if err := suite.ipam.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
			suite.T().Logf("unexpected error starting ipam server: %s", err)
		}
	}()

	ipamClient := ipam.NewIPAM(suite.logger, util.TestIPAMClientAddr)

	fflags := fflags.NewFFlags(suite.logger)
	suite.api, err = NewAPI(context.Background(), suite.logger, db, ipamClient, fflags)
	if err != nil {
		suite.T().Fatal(err)
	}
}

func (suite *HandlerTestSuite) BeforeTest(_, _ string) {
	suite.api.db.Exec("DELETE FROM users")
	suite.api.db.Exec("DELETE FROM organizations")
	suite.api.db.Exec("DELETE FROM user_organization")
	suite.api.db.Exec("DELETE FROM devices")
	var err error
	suite.testOrganizationID, err = suite.api.createUserIfNotExists(context.Background(), TestUserID, "testuser")
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
