// Assert response code is correct:
//
//	Then the response code should be 202
//
// Assert that a json field of the response body is correct.  This uses a http://github.com/itchyny/gojq expression to select the json field of the
// response:
//
//	Then the ".status" selection from the response should match "assigning"
//
// Assert that the response body matches the provided text:
//
//	Then the response should match "Hello"
//	Then the response should match:
//	"""
//	Hello
//	"""
//
// Assert that response json matches the provided json.  Differences in json formatting and field order are ignored.:
//
//	Then the response should match json:
//	  """
//	  {
//	      "id": "${cid}",
//	  }
//	  """
//
// Stores a json field of the response body in a scenario variable:
//
//	Given I store the ".id" selection from the response as ${cid}
//
// Stores a json value in a scenario variable:
//
//	Given I store json as ${$input}:
//	  """
//	  {
//	      "id": "${cid}",
//	  }
//	  """
//
// Assert that a response header matches the provided text:
//
//	Then the response header "Content-Type" should match "application/json;stream=watch"
//
// Assert that a json field of the response body is correct matches the provided json:
//
//	Then the ".deployment_location" selection from the response should match json:
//	  """
//	  {
//	      "namespace_id": "default"
//	  }
//	  """
package cucumber

import (
	"encoding/json"
	"fmt"

	"github.com/cucumber/godog"
	"github.com/itchyny/gojq"
	"github.com/pmezard/go-difflib/difflib"
)

func init() {
	StepModules = append(StepModules, func(ctx *godog.ScenarioContext, s *TestScenario) {
		ctx.Step(`^the response code should be (\d+)$`, s.theResponseCodeShouldBe)
		ctx.Step(`^the response should match json:$`, s.TheResponseShouldMatchJsonDoc)
		ctx.Step(`^the response should match:$`, s.theResponseShouldMatchText)
		ctx.Step(`^the response should match "([^"]*)"$`, s.theResponseShouldMatchText)
		ctx.Step(`^I store the "([^"]*)" selection from the response as \${([^"]*)}$`, s.iStoreTheSelectionFromTheResponseAs)
		ctx.Step(`^I store json as \${([^"]*)}:$`, s.iStoreJsonAsInput)
		ctx.Step(`^the "(.*)" selection from the response should match "([^"]*)"$`, s.theSelectionFromTheResponseShouldMatch)
		ctx.Step(`^the response header "([^"]*)" should match "([^"]*)"$`, s.theResponseHeaderShouldMatch)
		ctx.Step(`^the "([^"]*)" selection from the response should match json:$`, s.theSelectionFromTheResponseShouldMatchJson)
	})
}

func (s *TestScenario) theResponseCodeShouldBe(expected int) error {
	session := s.Session()
	actual := session.Resp.StatusCode
	if expected != actual {
		return fmt.Errorf("expected response code to be: %d, but actual is: %d, body: %s", expected, actual, string(session.RespBytes))
	}
	return nil
}

func (s *TestScenario) TheResponseShouldMatchJsonDoc(expected *godog.DocString) error {
	return s.theResponseShouldMatchJson(expected.Content)
}

func (s *TestScenario) theResponseShouldMatchJson(expected string) error {
	session := s.Session()

	if len(session.RespBytes) == 0 {
		return fmt.Errorf("got an empty response from server, expected a json body")
	}

	return s.JsonMustMatch(string(session.RespBytes), expected, true)
}

func (s *TestScenario) theResponseShouldMatchText(expected string) error {
	session := s.Session()

	expanded, err := s.Expand(expected, []string{"defs", "ref"})
	if err != nil {
		return err
	}

	actual := string(session.RespBytes)
	if expanded != actual {
		diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(expanded),
			B:        difflib.SplitLines(actual),
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

func (s *TestScenario) theResponseHeaderShouldMatch(header, expected string) error {
	session := s.Session()
	expanded, err := s.Expand(expected, []string{"defs", "ref"})
	if err != nil {
		return err
	}

	actual := session.Resp.Header.Get(header)
	if expanded != actual {
		return fmt.Errorf("reponse header '%s' does not match expected: %v, actual: %v", header, expanded, actual)
	}
	return nil
}

func (s *TestScenario) iStoreTheSelectionFromTheResponseAs(selector string, as string) error {

	session := s.Session()
	doc, err := session.RespJson()
	if err != nil {
		return err
	}

	query, err := gojq.Parse(selector)
	if err != nil {
		return err
	}

	iter := query.Run(doc)
	if next, found := iter.Next(); found {
		switch next.(type) {
		case map[interface{}]interface{}, []interface{}:
			bytes, err := json.Marshal(next)
			if err != nil {
				return err
			}
			s.Variables[as] = string(bytes)
		default:
			s.Variables[as] = fmt.Sprintf("%v", next)
		}
		return nil
	}
	return fmt.Errorf("expected JSON does not have node that matches selector: %s", selector)
}

func (s *TestScenario) iStoreJsonAsInput(as string, value *godog.DocString) error {
	content, err := s.Expand(value.Content, []string{"defs", "ref"})
	if err != nil {
		return err
	}
	m := map[string]interface{}{}
	err = json.Unmarshal([]byte(content), &m)
	if err != nil {
		return err
	}

	s.Variables[as] = m
	return nil
}

func (s *TestScenario) theSelectionFromTheResponseShouldMatch(selector string, expected string) error {
	session := s.Session()
	doc, err := session.RespJson()
	if err != nil {
		return err
	}

	query, err := gojq.Parse(selector)
	if err != nil {
		return err
	}

	expected, err = s.Expand(expected, []string{"defs", "ref"})
	if err != nil {
		return err
	}

	iter := query.Run(doc)
	if actual, found := iter.Next(); found {
		if actual == nil {
			actual = "null" // use null to represent missing value
		} else {
			actual = fmt.Sprintf("%v", actual)
		}
		if actual != expected {
			return fmt.Errorf("selected JSON does not match. expected: %v, actual: %v", expected, actual)
		}
		return nil
	}
	return fmt.Errorf("expected JSON does not have node that matches selector: %s", selector)
}

func (s *TestScenario) theSelectionFromTheResponseShouldMatchJson(selector string, expected *godog.DocString) error {

	session := s.Session()
	doc, err := session.RespJson()
	if err != nil {
		return err
	}

	query, err := gojq.Parse(selector)
	if err != nil {
		return err
	}

	iter := query.Run(doc)
	if actual, found := iter.Next(); found {
		actual, err := json.Marshal(actual)
		if err != nil {
			return err
		}

		return s.JsonMustMatch(string(actual), expected.Content, true)
	}
	return fmt.Errorf("expected JSON does not have node that matches selector: %s", selector)
}
