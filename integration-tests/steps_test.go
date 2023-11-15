package integration_tests

import (
	"context"
	"fmt"
	"github.com/cucumber/godog"
	"github.com/docker/docker/api/types/container"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/cucumber"
	"github.com/nexodus-io/nexodus/internal/wgcrypto"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"path"
	"strings"
)

type extender struct {
	*cucumber.TestScenario
	cleanups []func()
	helper   *Helper
}

func init() {
	cucumber.StepModules = append(cucumber.StepModules, func(ctx *godog.ScenarioContext, s *cucumber.TestScenario) {
		e := &extender{
			TestScenario: s,
			helper:       NewHelper(s.Suite.TestingT),
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
		ctx.Step(`^I generate a new key pair as \${([^}]*)}/\${([^}]*)}$`, e.iGenerateANewPublicKeyPairAsVariable)
		ctx.Step(`^I decrypt the sealed "([^"]*)" with "([^"]*)" and store the result as \${([^}]*)}$`, e.iDeycryptTheSealedWithAndStoreTheResultAsDevice_bearer_token)
		ctx.Step(`^I run playwright script "([^"]*)"$`, e.iRunPlaywrightScript)
		ctx.Step(`^I port forward to kube resource "([^"]*)" on port (\d+) via local port \${([^}]*)}$`, e.iStartPortForward)

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
	expanded, err := s.Expand(value)
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

func (s *extender) iGenerateANewPublicKeyPairAsVariable(privateKeyName string, publicKeyName string) error {
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}
	s.Variables[privateKeyName] = privateKey.String()
	s.Variables[publicKeyName] = privateKey.PublicKey().String()
	return nil
}

func (s *extender) iDeycryptTheSealedWithAndStoreTheResultAsDevice_bearer_token(sealedStr, privateKey, storeAs string) error {

	expanded, err := s.Expand(privateKey)
	if err != nil {
		return err
	}
	key, err := wgtypes.ParseKey(expanded)
	if err != nil {
		return err
	}

	expanded, err = s.Expand(sealedStr)
	if err != nil {
		return err
	}
	sealed, err := wgcrypto.ParseSealed(expanded)
	if err != nil {
		return err
	}
	value, err := sealed.Open(key[:])
	if err != nil {
		return err
	}
	s.Variables[storeAs] = string(value)
	return nil
}

func (s *extender) iRunPlaywrightScript(script string) error {
	name := s.Suite.TestingT.Name() + "-" + uuid.New().String()
	name = strings.ReplaceAll(name, "/", "-")

	certsDir, err := findCertsDir()
	require.NoError(s.Suite.TestingT, err)
	projectDir := path.Join(certsDir, "..")

	s.Suite.Mu.Lock()
	user := s.Users[s.CurrentUser]
	s.Suite.Mu.Unlock()

	container, err := testcontainers.GenericContainer(s.Suite.Context, testcontainers.GenericContainerRequest{
		ProviderType: providerType,
		ContainerRequest: testcontainers.ContainerRequest{
			// Too bad the following does not work on my mac..
			//FromDockerfile: testcontainers.FromDockerfile{
			//	Context:    projectDir,
			//	Dockerfile: "Containerfile.playwright",
			//	Repo:       "quay.io",
			//	Tag:        "nexodus/playwright:latest",
			//},
			Image:    "quay.io/nexodus/playwright:latest",
			Name:     name,
			Networks: []string{defaultNetwork},
			HostConfigModifier: func(hostConfig *container.HostConfig) {
				hostConfig.ExtraHosts = []string{
					fmt.Sprintf("try.nexodus.127.0.0.1.nip.io:%s", hostDNSName),
					fmt.Sprintf("api.try.nexodus.127.0.0.1.nip.io:%s", hostDNSName),
					fmt.Sprintf("auth.try.nexodus.127.0.0.1.nip.io:%s", hostDNSName),
				}
				hostConfig.AutoRemove = false
			},
			Mounts: []testcontainers.ContainerMount{
				{
					Source: testcontainers.GenericBindMountSource{
						HostPath: certsDir,
					},
					Target:   "/.certs",
					ReadOnly: true,
				},
				{
					Source: testcontainers.GenericBindMountSource{
						HostPath: path.Join(projectDir, "ui"),
					},
					Target:   "/ui",
					ReadOnly: false,
				},
			},
			// User: fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
			Env: map[string]string{
				"CI":               "true",
				"NEXODUS_USERNAME": user.Subject,
				"NEXODUS_PASSWORD": user.Password,
			},
			Cmd: []string{
				"/update-ca.sh",
				"/bin/bash",
				"-c",
				fmt.Sprintf("npm install && npx playwright test '%s'", script),
			},
			ConfigModifier: func(config *container.Config) {
				config.WorkingDir = "/ui"
			},
		},
		Started: true,
	})
	if err != nil {
		return err
	}
	defer func() {
		go func() {
			_ = container.Terminate(s.Suite.Context)
		}()
	}()
	container.FollowOutput(FnConsumer{
		Apply: func(l testcontainers.Log) {
			text := string(l.Content)
			s.Logf("%s", text)
		},
	})
	err = container.StartLogProducer(s.Suite.Context)
	if err != nil {
		return err
	}

	err = wait.ForExit().WaitUntilReady(s.Suite.Context, container)
	if err != nil {
		return err
	}

	state, err := container.State(s.Suite.Context)
	if err != nil {
		return err
	}
	if state.ExitCode != 0 {
		return fmt.Errorf("playwright failed with exit code %d", state.ExitCode)
	}
	return nil
}

func (s *extender) iStartPortForward(service string, port int, variable string) error {
	localPort, cleanup := s.helper.StartPortForward(s.Suite.Context, service, port)
	s.cleanups = append(s.cleanups, cleanup)
	s.Variables[variable] = localPort
	return nil
}
