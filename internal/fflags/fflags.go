package fflags

import (
	"fmt"
	"os"
	"strconv"

	"go.uber.org/zap"
)

// This implementation a hack to be replaced later with a proper feature
// flags backend. We will also need to pass in additional api user info
// into this API so that the answer can differ depending on who is asking.
// This is needed to allow admin-only access, or only partial rollouts of
// features, for example.
type FFlags struct {
	logger *zap.SugaredLogger
}

type FFlag struct {
	env          string
	defaultValue bool
}

var hardCodedFlags = map[string]FFlag{
	"multi-zone": {"APEX_FFLAG_MULTI_ZONE", false},
}

func NewFFlags(logger *zap.SugaredLogger) *FFlags {
	return &FFlags{
		logger: logger,
	}
}

func (f *FFlags) getFlagValue(fflag FFlag) bool {
	if envValue, err := strconv.ParseBool(os.Getenv(fflag.env)); err == nil {
		return envValue
	}
	return fflag.defaultValue
}

// ListFlags returns a map of all currently defined feature flags and
// whether those features are enabled (true) or not (false).
func (f *FFlags) ListFlags() map[string]bool {
	result := map[string]bool{}
	for name, fflag := range hardCodedFlags {
		result[name] = f.getFlagValue(fflag)
	}
	return result
}

// GetFlag returns whether the feature named by the string parameter
// flag is enabled (true) or not (false). An error is returned if
// the flag name is invalid.
func (f *FFlags) GetFlag(flag string) (bool, error) {
	fflag, ok := hardCodedFlags[flag]
	if !ok {
		f.logger.Errorf("Invalid feature flag name: %s", flag)
		return false, fmt.Errorf("Invalid feature flag name: %s", flag)
	}
	return f.getFlagValue(fflag), nil
}
