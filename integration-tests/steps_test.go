package integration_tests

import (
	"context"
	"fmt"
	"github.com/cucumber/godog"
	"github.com/nexodus-io/nexodus/internal/cucumber"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type extender struct {
	*cucumber.TestScenario
	cleanups []func()
}

func init() {
	cucumber.StepModules = append(cucumber.StepModules, func(ctx *godog.ScenarioContext, s *cucumber.TestScenario) {
		e := &extender{
			TestScenario: s,
		}
		ctx.Step(`^a user named "([^"]*)" with password "([^"]*)"$`, e.aUserNamedWithPassword)

		//ctx.Step(`^a user named "([^"]*)"$`, s.Suite.createUserNamed)
		//ctx.Step(`^a user named "([^"]*)" in organization "([^"]*)"$`, s.Suite.createUserNamedInOrganization)
		//ctx.Step(`^an org admin user named "([^"]*)"$`, s.Suite.createOrgAdminUserNamed)
		//ctx.Step(`^an org admin user named "([^"]*)" in organization "([^"]*)"$`, s.Suite.createOrgAdminUserNamedInOrganization)
		//ctx.Step(`^an admin user named "([^"]+)" with roles "([^"]+)"$`, s.Suite.createAdminUserNamed)

		ctx.Step(`^I am logged in as "([^"]*)"$`, e.iAmLoggedInAs)
		ctx.Step(`^I am not logged in$`, e.iAmNotLoggedIn)
		ctx.Step(`^I set the "([^"]*)" header to "([^"]*)"$`, e.iSetTheHeaderTo)
		ctx.Step(`^I store userid for "([^"]+)" as \${([^}]*)}$`, e.storeUserId)
		ctx.Step(`^I generate a new public key as \${([^}]*)}$`, e.iGenerateANewPublicKeyAsVariable)

		ctx.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
			if err == nil {
				for _, cleanup := range e.cleanups {
					cleanup()
				}
				e.cleanups = nil
			}
			return ctx, nil
		})
	})
}

func (s *extender) aUserNamedWithPassword(username string, password string) error {
	// users are shared concurrently across scenarios. so lock while we create the user...
	s.Suite.Mu.Lock()
	defer s.Suite.Mu.Unlock()

	// user already exists...
	if s.Users[username] != nil {
		return nil
	}
	ctx := s.Suite.Context
	userId, cleanup, err := createNewUserWithName(ctx, username, password)
	if err != nil {
		return err
	}
	s.cleanups = append(s.cleanups, cleanup)

	s.Users[username] = &cucumber.TestUser{
		Name:     username,
		Password: password,
		Subject:  userId,
	}
	return nil
}

func (s *extender) storeUserId(name, varName string) error {
	s.Suite.Mu.Lock()
	defer s.Suite.Mu.Unlock()
	user := s.Users[name]
	if user != nil {
		s.Variables[varName] = user.Subject
	}
	return nil
}

func (s *extender) iAmLoggedInAs(username string) error {
	s.Suite.Mu.Lock()
	user := s.Users[username]
	s.Suite.Mu.Unlock()

	if user == nil {
		return fmt.Errorf("previous step has not defined user: %s", username)
	}

	// do the oauth login...
	ctx := s.Suite.Context
	var err error
	user.Token, err = getOauth2Token(ctx, user.Subject, user.Password)
	if err != nil {
		return err
	}
	s.Session().Header.Del("Authorization")
	s.CurrentUser = username
	return nil
}

func (s *extender) iAmNotLoggedIn() {
	s.CurrentUser = "not-logged-in"
}

func (s *extender) iSetTheHeaderTo(name string, value string) error {
	expanded, err := s.Expand(value, []string{})
	if err != nil {
		return err
	}

	s.Session().Header.Set(name, expanded)
	return nil
}

func (s *extender) iGenerateANewPublicKeyAsVariable(name string) error {
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}
	s.Variables[name] = privateKey.PublicKey().String()
	return nil
}
