package integration_tests

import (
	"github.com/nexodus-io/nexodus/internal/stun"
	"os"
	"testing"
)

var testStunServer1Port = 0
var testStunServer2Port = 0

func TestMain(m *testing.M) {
	// code here gets run before the test suite executes...
	stunServer1, err := stun.ListenAndStart(":0", nil)
	if err != nil {
		panic(err)
	}
	defer stunServer1.Close()
	testStunServer1Port = stunServer1.Port

	stunServer2, err := stun.ListenAndStart(":0", nil)
	if err != nil {
		panic(err)
	}
	defer stunServer2.Close()
	testStunServer2Port = stunServer2.Port

	os.Exit(m.Run())
}
