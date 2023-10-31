package wgcrypto_test

import (
	"fmt"
	"github.com/nexodus-io/nexodus/internal/wgcrypto"
	"github.com/stretchr/testify/require"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"testing"
)

func TestDeviceTokenEncryption(t *testing.T) {

	wgKey, err := wgtypes.GenerateKey()
	require.NoError(t, err)
	publicKey := wgKey.PublicKey()

	message := []byte("hello world")
	originalSealed, err := wgcrypto.SealV1(publicKey[:], message)
	require.NoError(t, err)

	data := originalSealed.String()
	fmt.Println(data)
	parsedSealed, err := wgcrypto.ParseSealed(data)
	require.NoError(t, err)

	require.Equal(t, originalSealed, parsedSealed)

	actual, err := parsedSealed.Open(wgKey[:])
	require.NoError(t, err)

	require.Equal(t, message, actual)
}
