//go:build integration

package integration_tests

import (
	"context"
	"errors"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/nexodus-io/nexodus/internal/client"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"testing"
	"time"
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

	c, err := client.NewAPIClient(ctx, "https://api.try.nexodus.127.0.0.1.nip.io", nil, client.WithPasswordGrant(
		username,
		password,
	))
	require.NoError(err)
	user, _, err := c.UsersApi.GetUser(ctx, "me").Execute()
	require.NoError(err)
	orgs, _, err := c.OrganizationsApi.ListOrganizations(ctx).Execute()
	require.NoError(err)

	privateKey, err := wgtypes.GeneratePrivateKey()
	require.NoError(err)
	publicKey := privateKey.PublicKey().String()

	device, _, err := c.DevicesApi.CreateDevice(ctx).Device(public.ModelsAddDevice{
		EndpointLocalAddressIp4: "172.17.0.3",
		Hostname:                "bbac3081d5e8",
		OrganizationId:          orgs[0].Id,
		PublicKey:               publicKey,
		UserId:                  user.Id,
		Endpoints: []public.ModelsEndpoint{
			{
				Source:   "local",
				Address:  "172.17.0.3:58664",
				Distance: 0,
			},
			{
				Source:   "stun:",
				Address:  "47.196.141.165",
				Distance: 12,
			},
		},
	}).Execute()
	require.NoError(err)

	_, resp, err := c.DevicesApi.CreateDevice(ctx).Device(public.ModelsAddDevice{
		EndpointLocalAddressIp4: "172.17.0.3",
		Hostname:                "bbac3081d5e8",
		OrganizationId:          orgs[0].Id,
		PublicKey:               publicKey,
		UserId:                  user.Id,
		Endpoints: []public.ModelsEndpoint{
			{
				Source:   "local",
				Address:  "172.17.0.3:58664",
				Distance: 0,
			},
			{
				Source:   "stun:",
				Address:  "47.196.141.165",
				Distance: 12,
			},
		},
	}).Execute()
	require.Error(err)
	require.NotNil(resp)

	var apiError *public.GenericOpenAPIError
	require.True(errors.As(err, &apiError))

	conflict, ok := apiError.Model().(public.ModelsConflictsError)
	require.True(ok)
	require.Equal(device.Id, conflict.Id)
}

func TestWatchDevices(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	assert := helper.assert
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cancel := helper.createNewUser(ctx, password)
	defer cancel()

	c, err := client.NewAPIClient(ctx, "https://api.try.nexodus.127.0.0.1.nip.io", nil, client.WithPasswordGrant(
		username,
		password,
	))
	require.NoError(err)
	user, _, err := c.UsersApi.GetUser(ctx, "me").Execute()
	require.NoError(err)
	orgs, _, err := c.OrganizationsApi.ListOrganizations(ctx).Execute()
	require.NoError(err)

	privateKey, err := wgtypes.GeneratePrivateKey()
	require.NoError(err)
	publicKey := privateKey.PublicKey().String()

	watch, _, err := c.DevicesApi.ListDevicesInOrganization(ctx, orgs[0].Id).Watch()
	require.NoError(err)
	defer watch.Close()

	kind, _, err := watch.Receive()
	require.NoError(err)
	assert.Equal("bookmark", kind)

	device, _, err := c.DevicesApi.CreateDevice(ctx).Device(public.ModelsAddDevice{
		EndpointLocalAddressIp4: "172.17.0.3",
		Hostname:                "bbac3081d5e8",
		OrganizationId:          orgs[0].Id,
		PublicKey:               publicKey,
		UserId:                  user.Id,
		Endpoints: []public.ModelsEndpoint{
			{
				Source:   "local",
				Address:  "172.17.0.3:58664",
				Distance: 0,
			},
			{
				Source:   "stun:",
				Address:  "47.196.141.165",
				Distance: 12,
			},
		},
	}).Execute()
	require.NoError(err)

	// We should get sent an event for the device that was created
	kind, watchedDevice, err := watch.Receive()
	require.NoError(err)
	assert.Equal("change", kind)
	assert.Equal(*device, watchedDevice)

	device, _, err = c.DevicesApi.DeleteDevice(ctx, device.Id).Execute()
	require.NoError(err)

	// We should get sent an event for the device that was deleted
	kind, watchedDevice, err = watch.Receive()
	require.NoError(err)
	assert.Equal("delete", kind)
	assert.Equal(*device, watchedDevice)

}

func TestConcurrentApiAccess(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	c, err := client.NewAPIClient(ctx, "https://api.try.nexodus.127.0.0.1.nip.io", nil, client.WithPasswordGrant(
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
