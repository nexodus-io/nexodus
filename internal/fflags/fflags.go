package fflags

import (
	"fmt"
	"github.com/gin-gonic/gin"
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
	Flags  map[string]func() bool
}

func NewFFlags(logger *zap.SugaredLogger) *FFlags {
	return &FFlags{
		logger: logger,
		Flags:  map[string]func() bool{},
	}
}
func (f *FFlags) RegisterEnvFlag(name, env string, defaultValue bool) {
	result := defaultValue
	if envValue, err := strconv.ParseBool(os.Getenv(env)); err == nil {
		result = envValue
	}
	f.RegisterFlag(name, func() bool {
		return result
	})
}

func (f *FFlags) RegisterFlag(name string, fn func() bool) {
	f.Flags[name] = fn
}

func (f *FFlags) getFlagValue(c *gin.Context, name string, fn func() bool) bool {
	ctxName := fmt.Sprintf("nexodus.fflag.%s", name)
	if _, found := c.Get(ctxName); found {
		return c.GetBool(ctxName)
	}
	return fn()
}

// ListFlags returns a map of all currently defined feature flags and
// whether those features are enabled (true) or not (false).
func (f *FFlags) ListFlags(c *gin.Context) map[string]bool {
	result := map[string]bool{}
	for name, fn := range f.Flags {
		result[name] = f.getFlagValue(c, name, fn)
	}
	return result
}

// GetFlag returns whether the feature named by the string parameter
// flag is enabled (true) or not (false). An error is returned if
// the flag name is invalid.
func (f *FFlags) GetFlag(c *gin.Context, flag string) (bool, error) {
	fn, ok := f.Flags[flag]
	if !ok {
		return false, fmt.Errorf("invalid feature flag name: %s", flag)
	}
	return f.getFlagValue(c, flag, fn), nil
}
