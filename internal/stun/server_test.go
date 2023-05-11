package stun_test

import (
	"fmt"
	"github.com/nexodus-io/nexodus/internal/stun"
	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"testing"
)

func TestListenAndStart(t *testing.T) {
	require := require.New(t)
	log, err := zap.NewDevelopment()
	require.NoError(err)
	server, err := stun.ListenAndStart("0.0.0.0:0", log)
	require.NoError(err)
	defer util.IgnoreError(server.Shutdown)

	_, err = stun.Request(log.Sugar(), fmt.Sprintf("127.0.0.1:%d", server.Port), 0)
	require.NoError(err)
}
