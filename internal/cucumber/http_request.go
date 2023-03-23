// Setting a path prefixed to subsequent http requests:
//
//	Given the path prefix is "/api/kafkas_mgmt"
//
// Send an http request. Supports (GET|POST|PUT|DELETE|PATCH|OPTION):
//
//	When I GET path "/v1/some/${kid}
//
// Send an http request with a body. Supports (GET|POST|PUT|DELETE|PATCH|OPTION):
//
//	When I POST path "/v1/some/${kid}" with json body:
//	  """
//	  {"some":"${kid}"}
//	  """
//
// Wait until an http get responds with an expected result or a timeout occurs:
//
//	Given I wait up to "35.5" seconds for a GET on path "/v1/some/path" response ".total" selection to match "1"
//
// Wait until an http get responds with an expected response code or a timeout occurs:
//
//	Given I wait up to "35.5" seconds for a GET on path "/v1/some/path" response code to match "200"
//
// Send an http request that receives a stream of events. Supports (GET|POST|PUT|DELETE|PATCH|OPTION). :
//
//	When I GET path "/v1/some/${kid} as an event stream
//
// Wait until a json event arrives on the event stream or a timeout occurs:
//
//	Given I wait up to "35" seconds for a response json event
package cucumber

import (
	"bytes"
	"context"
	"encoding/json"
	fmt "fmt"
	"io"
	"net/http"
	"time"

	"github.com/cucumber/godog"
)

func init() {
	StepModules = append(StepModules, func(ctx *godog.ScenarioContext, s *TestScenario) {
		ctx.Step(`^the path prefix is "([^"]*)"$`, s.theApiPrefixIs)
		ctx.Step(`^I (GET|POST|PUT|DELETE|PATCH|OPTION) path "([^"]*)"$`, s.sendHttpRequest)
		ctx.Step(`^I (GET|POST|PUT|DELETE|PATCH|OPTION) path "([^"]*)" as a json event stream$`, s.sendHttpRequestAsEventStream)
		ctx.Step(`^I (GET|POST|PUT|DELETE|PATCH|OPTION) path "([^"]*)" with json body:$`, s.SendHttpRequestWithJsonBody)
		ctx.Step(`^I wait up to "([^"]*)" seconds for a GET on path "([^"]*)" response "([^"]*)" selection to match "([^"]*)"$`, s.iWaitUpToSecondsForAGETOnPathResponseSelectionToMatch)
		ctx.Step(`^I wait up to "([^"]*)" seconds for a GET on path "([^"]*)" response code to match "([^"]*)"$`, s.iWaitUpToSecondsForAGETOnPathResponseCodeToMatch)
		ctx.Step(`^I wait up to "([^"]*)" seconds for a response event$`, s.iWaitUpToSecondsForAResponseJsonEvent)
	})
}

func (s *TestScenario) theApiPrefixIs(prefix string) error {
	s.PathPrefix = prefix
	return nil
}

func (s *TestScenario) sendHttpRequest(method, path string) error {
	return s.SendHttpRequestWithJsonBody(method, path, nil)
}

func (s *TestScenario) sendHttpRequestAsEventStream(method, path string) error {
	return s.SendHttpRequestWithJsonBodyAndStyle(method, path, nil, true, true)
}

func (s *TestScenario) SendHttpRequestWithJsonBody(method, path string, jsonTxt *godog.DocString) (err error) {
	return s.SendHttpRequestWithJsonBodyAndStyle(method, path, jsonTxt, false, true)
}

func (s *TestScenario) SendHttpRequestWithJsonBodyAndStyle(method, path string, jsonTxt *godog.DocString, eventStream bool, expandJson bool) (err error) {
	// handle panic
	defer func() {
		switch t := recover().(type) {
		case string:
			err = fmt.Errorf(t)
		case error:
			err = t
		}
	}()

	session := s.Session()

	body := &bytes.Buffer{}
	if jsonTxt != nil {
		expanded := jsonTxt.Content
		if expandJson {
			expanded, err = s.Expand(expanded, []string{})
			if err != nil {
				return err
			}
		}
		body.WriteString(expanded)
	}
	expandedPath, err := s.Expand(path, []string{})
	if err != nil {
		return err
	}
	fullUrl := s.Suite.ApiURL + s.PathPrefix + expandedPath

	// Lets reset all the response session state...
	if session.Resp != nil {
		_ = session.Resp.Body.Close()
	}
	session.EventStream = false
	session.Resp = nil
	session.RespBytes = nil
	session.respJson = nil

	ctx := session.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	req, err := http.NewRequestWithContext(ctx, method, fullUrl, body)
	if err != nil {
		return err
	}

	// We consume the session headers on every request except for the Authorization header.
	req.Header = session.Header
	session.Header = http.Header{}

	if req.Header.Get("Authorization") != "" {
		session.Header.Set("Authorization", req.Header.Get("Authorization"))
	} else if session.TestUser != nil && session.TestUser.Token != nil {
		req.Header.Set("Authorization", "Bearer "+session.TestUser.Token.AccessToken)
	}

	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := session.Client.Do(req)
	if err != nil {
		return err
	}

	session.Resp = resp
	session.EventStream = eventStream
	if !eventStream {
		defer func() {
			_ = resp.Body.Close()
		}()

		session.RespBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
	} else {

		c := make(chan interface{})
		session.EventStreamEvents = c
		go func() {
			d := json.NewDecoder(session.Resp.Body)
			defer func() {
				_ = resp.Body.Close()
			}()

			for {
				var event interface{}
				err := d.Decode(&event)
				if err != nil {
					close(c)
					return
				}
				c <- event
			}
		}()
	}

	return nil
}

func (s *TestScenario) iWaitUpToSecondsForAResponseJsonEvent(timeout float64) error {
	session := s.Session()
	if !session.EventStream {
		return fmt.Errorf("the last http request was not performed as a json event stream")
	}

	session.respJson = nil
	session.RespBytes = session.RespBytes[0:0]

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout*float64(time.Second)))
	defer cancel()

	select {
	case event := <-session.EventStreamEvents:

		session.respJson = event
		var err error
		session.RespBytes, err = json.Marshal(event)
		if err != nil {
			return err
		}
	case <-ctx.Done():
	}

	return nil
}

func (s *TestScenario) iWaitUpToSecondsForAGETOnPathResponseCodeToMatch(timeout float64, path string, expected int) error {

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout*float64(time.Second)))
	defer cancel()
	session := s.Session()
	session.Ctx = ctx
	defer func() {
		session.Ctx = nil
	}()

	for {
		err := s.sendHttpRequest("GET", path)
		if err == nil {
			err = s.theResponseCodeShouldBe(expected)
			if err == nil {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return nil
		default:
			time.Sleep(time.Duration(timeout * float64(time.Second) / 10.0))
		}
	}
}

func (s *TestScenario) iWaitUpToSecondsForAGETOnPathResponseSelectionToMatch(timeout float64, path string, selection, expected string) error {

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout*float64(time.Second)))
	defer cancel()
	session := s.Session()
	session.Ctx = ctx
	defer func() {
		session.Ctx = nil
	}()

	for {
		err := s.sendHttpRequest("GET", path)
		if err == nil {
			err = s.theSelectionFromTheResponseShouldMatch(selection, expected)
			if err == nil {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return nil
		default:
			time.Sleep(time.Duration(timeout * float64(time.Second) / 10.0))
		}
	}
}
