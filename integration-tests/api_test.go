//go:build integration

package integration_tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nexodus-io/nexodus/internal/client"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestApiClientConflictError(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	c, err := client.NewClient(ctx, "https://api.try.nexodus.127.0.0.1.nip.io", nil, client.WithPasswordGrant(
		username,
		password,
	))
	require.NoError(err)
	orgs, _, err := c.OrganizationsApi.ListOrganizations(ctx).Execute()
	require.NoError(err)

	privateKey, err := wgtypes.GeneratePrivateKey()
	require.NoError(err)
	publicKey := privateKey.PublicKey().String()

	device, _, err := c.DevicesApi.CreateDevice(ctx).Device(client.ModelsAddDevice{
		Hostname:  client.PtrString("bbac3081d5e8"),
		VpcId:     orgs[0].Id,
		PublicKey: client.PtrString(publicKey),
		Endpoints: []client.ModelsEndpoint{
			{
				Source:  client.PtrString("local"),
				Address: client.PtrString("172.17.0.3:58664"),
			},
			{
				Source:  client.PtrString("stun:"),
				Address: client.PtrString("47.196.141.165"),
			},
		},
	}).Execute()
	require.NoError(err)

	_, resp, err := c.DevicesApi.CreateDevice(ctx).Device(client.ModelsAddDevice{
		Hostname:  client.PtrString("bbac3081d5e8"),
		VpcId:     orgs[0].Id,
		PublicKey: client.PtrString(publicKey),
		Endpoints: []client.ModelsEndpoint{
			{
				Source:  client.PtrString("local"),
				Address: client.PtrString("172.17.0.3:58664"),
			},
			{
				Source:  client.PtrString("stun:"),
				Address: client.PtrString("47.196.141.165"),
			},
		},
	}).Execute()
	require.Error(err)
	require.NotNil(resp)

	var apiError *client.GenericOpenAPIError
	require.True(errors.As(err, &apiError))

	conflict, ok := apiError.Model().(client.ModelsConflictsError)
	require.True(ok)
	require.Equal(device.GetId(), conflict.GetId())
}

func TestConcurrentApiAccess(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	c, err := client.NewClient(ctx, "https://api.try.nexodus.127.0.0.1.nip.io", nil, client.WithPasswordGrant(
		username,
		password,
	))
	require.NoError(err)

	concurrency := 20
	results := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			_, _, err := c.UsersApi.GetUser(ctx, "me").Execute()
			results <- err
		}()
	}
	for i := 0; i < concurrency; i++ {
		select {
		case <-ctx.Done():
			helper.require.Fail("test timeout")
			break
		case err := <-results:
			helper.require.NoError(err)
		}
	}

}

func TestDevicesInformer(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cancel := helper.createNewUser(ctx, password)
	defer cancel()

	c, err := client.NewClient(ctx, "https://api.try.nexodus.127.0.0.1.nip.io", nil, client.WithPasswordGrant(
		username,
		password,
	))
	require.NoError(err)
	orgs, _, err := c.OrganizationsApi.ListOrganizations(ctx).Execute()
	require.NoError(err)

	privateKey, err := wgtypes.GeneratePrivateKey()
	require.NoError(err)
	publicKey := privateKey.PublicKey().String()

	ctx = c.EventsApi.Watch(ctx).NewSharedInformerContext()
	sgInformer := c.VPCApi.ListSecurityGroupsInVPC(ctx, orgs[0].GetId()).Informer()
	devicesInformer := c.VPCApi.ListDevicesInVPC(ctx, orgs[0].GetId()).Informer()
	devicesChanged := func() bool {
		select {
		case <-devicesInformer.Changed():
			return true
		default:
		}
		return false
	}
	require.False(devicesChanged())

	devices, _, err := devicesInformer.Execute()
	require.NoError(err)
	require.Len(devices, 0)

	sgs, _, err := sgInformer.Execute()
	require.NoError(err)
	require.Len(sgs, 1)

	require.True(devicesChanged())

	device, _, err := c.DevicesApi.CreateDevice(ctx).Device(client.ModelsAddDevice{
		Hostname:  client.PtrString("bbac3081d5e8"),
		VpcId:     orgs[0].Id,
		PublicKey: client.PtrString(publicKey),
		Endpoints: []client.ModelsEndpoint{
			{
				Source:  client.PtrString("local"),
				Address: client.PtrString("172.17.0.3:58664"),
			},
			{
				Source:  client.PtrString("stun:"),
				Address: client.PtrString("47.196.141.165"),
			},
		},
	}).Execute()
	require.NoError(err)

	require.Eventually(devicesChanged, 2*time.Second, time.Millisecond)

	devices, _, err = devicesInformer.Execute()
	require.NoError(err)
	require.Len(devices, 1)

	// We should get s
	_, _, err = c.DevicesApi.DeleteDevice(ctx, device.GetId()).Execute()
	require.NoError(err)

	require.Eventually(devicesChanged, 2*time.Second, time.Millisecond)

	devices, _, err = devicesInformer.Execute()
	require.NoError(err)
	require.Len(devices, 0)

}
