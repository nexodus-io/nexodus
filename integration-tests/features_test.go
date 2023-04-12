//go:build integration

package integration_tests

import (
	"context"
	"github.com/cucumber/godog"
	"github.com/nexodus-io/nexodus/internal/cucumber"
	"github.com/stretchr/testify/require"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

func TestFeatures(t *testing.T) {

	// This looks for feature files in the current directory
	var cucumberOptions = cucumber.DefaultOptions()
	// configures where to look for feature files.
	cucumberOptions.Paths = []string{"features"}
	// output more info when test is run in verbose mode.
	for _, arg := range os.Args[1:] {
		if arg == "-test.v=true" || arg == "-test.v" || arg == "-v" { // go test transforms -v option
			cucumberOptions.Format = "pretty"
		}
	}

	tlsConfig := NewTLSConfig(t)
	require := require.New(t)

	for i := range cucumberOptions.Paths {
		root := cucumberOptions.Paths[i]

		err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {

			require.NoError(err)

			if info.IsDir() {
				return nil
			}

			name := filepath.Base(info.Name())
			ext := filepath.Ext(info.Name())

			if ext != ".feature" {
				return nil
			}

			shortName := strings.TrimSuffix(name, ext)
			t.Run(shortName, func(t *testing.T) {

				// To preserve the current behavior, the test are market to be "safely" run in parallel, however
				// we may think to introduce a new naming convention i.e. files that ends with _parallel would
				// cause t.Parallel() to be invoked, other tests won't, so they won't be executed concurrently.
				//
				// This could help reducing/removing the need of explicit lock
				t.Parallel()

				o := cucumberOptions
				o.TestingT = t
				o.Paths = []string{path.Join(root, name)}

				s := cucumber.NewTestSuite()
				s.Context = context.Background()
				s.ApiURL = "https://api.try.nexodus.127.0.0.1.nip.io"
				s.TlsConfig = tlsConfig

				status := godog.TestSuite{
					Name:                shortName,
					Options:             &o,
					ScenarioInitializer: s.InitializeScenario,
				}.Run()
				if status != 0 {
					t.Fail()
				}
			})
			return nil
		})
		require.NoError(err)
	}
}
