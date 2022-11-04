package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/redhat-et/apex/internal/ipam"
	"github.com/redhat-et/apex/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const (
	TestUserID = "f606de8d-092d-4606-b981-80ce9f5a3b2a"
)

type HandlerTestSuite struct {
	suite.Suite
	ipam *http.Server
	wg   *sync.WaitGroup
	api  *API
}

func (suite *HandlerTestSuite) SetupSuite() {
	db, err := util.NewTestDatabase()
	if err != nil {
		suite.T().Fatal(err)
	}

	suite.ipam = util.NewTestIPAMServer()
	suite.wg = &sync.WaitGroup{}
	suite.wg.Add(1)
	go func() {
		defer suite.wg.Done()
		_ = suite.ipam.ListenAndServe()
	}()

	ipamClient := ipam.NewIPAM(util.TestIPAMClientAddr)
	suite.api = NewAPI(db, ipamClient)
}

func (suite *HandlerTestSuite) BeforeTest(_, _ string) {
	suite.api.db.Exec("DELETE FROM users WHERE id <> ?", TestUserID)
	suite.api.db.Exec("DELETE FROM peers")
	suite.api.db.Exec("DELETE FROM zones WHERE name <> ?", "default")
	suite.api.db.Exec("DELETE FROM devices")
}

func (suite *HandlerTestSuite) ServeRequest(method, path, uri string, handler func(*gin.Context), body io.Reader) (*http.Request, *httptest.ResponseRecorder, error) {
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
	expectedPageSize := 24
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
