package memfm

import (
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr/tests"
	"testing"
)

func TestFetchManager(t *testing.T) {
	t.Parallel()
	tests.TestFetchManagerReducesDBFetchesAtTheTail(t, New())
}
