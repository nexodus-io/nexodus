// Package cucumber allows you to use cucumber to execute Gherkin based
// BDD test scenarios with some helpful API testing step implementations.
//
// Some steps allow you store variables or use those variables.  The variables
// are scoped to the Scenario.  The http response state is stored in the users
// session.  Switching users will switch the session.  Scenarios are executed
// concurrently.  The same user can be logged into two scenarios, but each scenario
// has a different session.
//
// Note: be careful using the same user/organization across different scenarios since
// they will likely see unexpected API mutations done in the other scenarios.
//
// Using in a test
//  func TestMain(m *testing.M) {
//
//	ocmServer := mocks.NewMockConfigurableServerBuilder().Build()
//	defer ocmServer.Close()
//
//	h, _, teardown := test.RegisterIntegration(&testing.T{}, ocmServer)
//	defer teardown()
//
//	cucumber.TestMain(h)
//
//}

package cucumber

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	jsonpatch "github.com/evanphx/json-patch"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
	"github.com/itchyny/gojq"
	"github.com/pmezard/go-difflib/difflib"
)

func NewTestSuite() *TestSuite {
	return &TestSuite{
		ApiURL:    "http://localhost:8000",
		nextOrgId: 20000000,
	}
}

func DefaultOptions() godog.Options {
	opts := godog.Options{
		Output:      colors.Colored(os.Stdout),
		Format:      "progress",
		Paths:       []string{"features"},
		Randomize:   time.Now().UTC().UnixNano(), // randomize TestScenario execution order
		Concurrency: 10,
	}

	return opts
}

// TestSuite holds the state global to all the test scenarios.
// It is accessed concurrently from all test scenarios.
type TestSuite struct {
	Context   context.Context
	ApiURL    string
	Mu        sync.Mutex
	nextOrgId uint32
	TlsConfig *tls.Config
	DB        *gorm.DB
	TestingT  *testing.T
}

// TestUser represents a user that can login to the system.  The same users are shared by
// the different test scenarios.
type TestUser struct {
	Name     string
	Subject  string
	Password string
	Token    *oauth2.Token
	Mu       sync.Mutex
}

// TestScenario holds that state of single scenario.  It is not accessed
// concurrently.
type TestScenario struct {
	Suite           *TestSuite
	CurrentUser     string
	PathPrefix      string
	sessions        map[string]*TestSession
	Variables       map[string]interface{}
	Users           map[string]*TestUser
	hasTestCaseLock bool
}

func (s *TestScenario) Logf(format string, args ...any) {
	s.Suite.TestingT.Logf(format, args...)
}

func (s *TestScenario) User() *TestUser {
	s.Suite.Mu.Lock()
	defer s.Suite.Mu.Unlock()
	return s.Users[s.CurrentUser]
}

func (s *TestScenario) Session() *TestSession {
	result := s.sessions[s.CurrentUser]
	if result == nil {
		result = &TestSession{
			TestUser: s.User(),
			Client: &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: s.Suite.TlsConfig,
				},
			},
			Header: http.Header{},
		}
		s.sessions[s.CurrentUser] = result
	}
	return result
}

func (s *TestScenario) JsonMustMatch(actual, expected string, expand bool) error {

	var actualParsed interface{}
	err := json.Unmarshal([]byte(actual), &actualParsed)
	if err != nil {
		return fmt.Errorf("error parsing actual json: %w\njson was:\n%s", err, actual)
	}

	var expectedParsed interface{}
	expanded := expected
	if expand {
		expanded, err = s.Expand(expected, "defs", "ref")
		if err != nil {
			return err
		}
	}

	// When you first set up a test step, you might not know what JSON you are expecting.
	if strings.TrimSpace(expanded) == "" {
		actual, _ := json.MarshalIndent(actualParsed, "", "  ")
		return fmt.Errorf("expected json not specified, actual json was:\n%s", actual)
	}

	if err := json.Unmarshal([]byte(expanded), &expectedParsed); err != nil {
		return fmt.Errorf("error parsing expected json: %w\njson was:\n%s", err, expanded)
	}

	if !reflect.DeepEqual(expectedParsed, actualParsed) {
		expected, _ := json.MarshalIndent(expectedParsed, "", "  ")
		actual, _ := json.MarshalIndent(actualParsed, "", "  ")

		diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(string(expected)),
			B:        difflib.SplitLines(string(actual)),
			FromFile: "Expected",
			FromDate: "",
			ToFile:   "Actual",
			ToDate:   "",
			Context:  1,
		})
		return fmt.Errorf("actual does not match expected, diff:\n%s", diff)
	}

	return nil
}

func (s *TestScenario) JsonMustContain(actual, expected string, expand bool) error {

	var actualParsed interface{}
	err := json.Unmarshal([]byte(actual), &actualParsed)
	if err != nil {
		return fmt.Errorf("error parsing actual json: %w\njson was:\n%s", err, actual)
	}

	if expand {
		expected, err = s.Expand(expected, "defs", "ref")
		if err != nil {
			return err
		}
	}

	// When you first set up a test step, you might not know what JSON you are expecting.
	if strings.TrimSpace(expected) == "" {
		actual, _ := json.MarshalIndent(actualParsed, "", "  ")
		return fmt.Errorf("expected json not specified, actual json was:\n%s", actual)
	}

	actualIndented, err := json.MarshalIndent(actualParsed, "", "  ")
	if err != nil {
		return err
	}

	merged, err := jsonpatch.MergeMergePatches(actualIndented, []byte(expected))
	if err != nil {
		return err
	}

	err = json.Unmarshal(merged, &actualParsed)
	if err != nil {
		return fmt.Errorf("error parsing merged json: %w\njson was:\n%s", err, actual)
	}
	mergedIndented, err := json.MarshalIndent(actualParsed, "", "  ")
	if err != nil {
		return err
	}

	if string(actualIndented) != string(mergedIndented) {

		diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(string(mergedIndented)),
			B:        difflib.SplitLines(string(actualIndented)),
			FromFile: "Expected",
			FromDate: "",
			ToFile:   "Actual",
			ToDate:   "",
			Context:  1,
		})
		return fmt.Errorf("actual does not match expected, diff:\n%s", diff)
	}

	return nil
}

// Expand replaces ${var} or $var in the string based on saved Variables in the session/test scenario.
func (s *TestScenario) Expand(value string, skippedVars ...string) (result string, rerr error) {
	return os.Expand(value, func(name string) string {
		if contains(skippedVars, name) {
			return "$" + name
		}
		res, err := s.ResolveString(name)
		if err != nil {
			rerr = err
			return ""
		}
		return res
	}), rerr
}

func (s *TestScenario) ResolveString(name string) (string, error) {

	value, err := s.Resolve(name)
	if err != nil {
		return "", err
	}

	switch value := value.(type) {
	case string:
		return value, nil
	case bool:
		if value {
			return "true", nil
		} else {
			return "false", nil
		}
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", value), nil
	case float32, float64:
		// handle int64 returned as float in json
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", value), "0"), "."), nil
	case nil:
		return "", nil
	case error:
		return "", fmt.Errorf("failed to evaluate selection: %s: %w", name, value)
	}

	bytes, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (s *TestScenario) Resolve(name string) (interface{}, error) {
	session := s.Session()
	if name == "response" {
		return session.RespJson()
	} else if strings.HasPrefix(name, "response.") || strings.HasPrefix(name, "response[") {
		selector := "." + name
		query, err := gojq.Parse(selector)
		if err != nil {
			return nil, err
		}

		j, err := session.RespJson()
		if err != nil {
			return nil, err
		}

		j = map[string]interface{}{
			"response": j,
		}

		iter := query.Run(j)
		if next, found := iter.Next(); found {
			return next, nil
		} else {
			return nil, fmt.Errorf("field ${%s} not found in json response:\n%s", name, string(session.RespBytes))
		}
	}
	value, found := s.Variables[name]
	if !found {
		return nil, fmt.Errorf("variable ${%s} not defined yet", name)
	}
	return value, nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// TestSession holds the http context for a user kinda like a browser.  Each scenario
// had a different session even if using the same user.
type TestSession struct {
	TestUser          *TestUser
	Client            *http.Client
	Resp              *http.Response
	Ctx               context.Context
	RespBytes         []byte
	respJson          interface{}
	Header            http.Header
	EventStream       bool
	EventStreamEvents chan interface{}
	Debug             bool
}

// RespJson returns the last http response body as json
func (s *TestSession) RespJson() (interface{}, error) {
	if s.respJson == nil {
		if err := json.Unmarshal(s.RespBytes, &s.respJson); err != nil {
			return nil, fmt.Errorf("error parsing json response: %w\nbody: %s", err, string(s.RespBytes))
		}

		if s.Debug {
			fmt.Println("response json:")
			e := json.NewEncoder(os.Stdout)
			e.SetIndent("", "  ")
			_ = e.Encode(s.respJson)
			fmt.Println("")
		}
	}
	return s.respJson, nil
}

func (s *TestSession) SetRespBytes(bytes []byte) {
	s.RespBytes = bytes
	s.respJson = nil
}

// StepModules is the list of functions used to add steps to a godog.ScenarioContext, you can
// add more to this list if you need test TestSuite specific steps.
var StepModules []func(ctx *godog.ScenarioContext, s *TestScenario)

func (suite *TestSuite) InitializeScenario(ctx *godog.ScenarioContext) {
	s := &TestScenario{
		Suite:     suite,
		Users:     map[string]*TestUser{},
		sessions:  map[string]*TestSession{},
		Variables: map[string]interface{}{},
	}

	for _, module := range StepModules {
		module(ctx, s)
	}
}
