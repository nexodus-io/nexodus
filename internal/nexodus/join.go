package nexodus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/nexodus-io/nexodus/internal/client"
)

func (nx *Nexodus) createOrUpdateDeviceOperation(userID string, endpoints []client.ModelsEndpoint) (client.ModelsDevice, string, error) {
	newDev := client.ModelsAddDevice{
		VpcId:           nx.vpc.Id,
		SecurityGroupId: client.PtrOptionalString(nx.securityGroupId),
		PublicKey:       &nx.wireguardPubKey,
		AdvertiseCidrs:  nx.advertiseCidrs,
		SymmetricNat:    &nx.symmetricNat,
		Hostname:        &nx.hostname,
		Relay:           client.PtrBool(nx.relay || nx.relayDerp),
		Os:              &nx.os,
		Endpoints:       endpoints,
	}

	if len(nx.requestedIP) > 0 {
		newDev.Ipv4TunnelIps = []client.ModelsTunnelIP{
			{
				Address: &nx.requestedIP,
				Cidr:    nx.vpc.Ipv4Cidr,
			},
		}
	}
	d, _, err := nx.client.DevicesApi.CreateDevice(context.Background()).Device(newDev).Execute()
	deviceOperationMsg := "Successfully registered device"
	var resp *http.Response
	if err != nil {
		var apiError *client.GenericOpenAPIError
		if errors.As(err, &apiError) {
			switch model := apiError.Model().(type) {
			case client.ModelsConflictsError:
				d, resp, err = nx.client.DevicesApi.UpdateDevice(context.Background(), model.GetId()).Update(client.ModelsUpdateDevice{
					AdvertiseCidrs:  newDev.AdvertiseCidrs,
					Endpoints:       newDev.Endpoints,
					Hostname:        newDev.Hostname,
					Relay:           newDev.Relay,
					SecurityGroupId: newDev.SecurityGroupId,
					SymmetricNat:    newDev.SymmetricNat,
					VpcId:           newDev.VpcId,
				}).Execute()
				deviceOperationMsg = "Reconnected as device"
				if err != nil {
					respText := ""
					if resp != nil {
						bytes, err := io.ReadAll(resp.Body)
						if err != nil {
							return client.ModelsDevice{}, "", fmt.Errorf("error updating device: %w - %s", err, resp.Status)
						}
						respText = string(bytes)
					}
					return client.ModelsDevice{}, "", fmt.Errorf("error updating device: %w - %s", err, respText)
				}
			default:
				return client.ModelsDevice{}, "", fmt.Errorf("error creating device: %w", err)
			}
		} else {
			return client.ModelsDevice{}, "", fmt.Errorf("error creating device: %w", err)
		}
	}

	resp, err = nx.updateDeviceRelayMetadata(d.GetId())
	if err != nil {
		respText := ""
		if resp != nil {
			bytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return client.ModelsDevice{}, "", fmt.Errorf("error updating device metadata: %w - %s", err, resp.Status)
			}
			respText = string(bytes)
		}
		return client.ModelsDevice{}, "", fmt.Errorf("error updating device metadata: %w - %s", err, respText)
	}

	return *d, deviceOperationMsg, nil
}

func (nx *Nexodus) updateDeviceRelayMetadata(deviceId string) (*http.Response, error) {
	if nx.relay || nx.relayDerp {
		var rtype interface{}
		if nx.relay {
			rtype = "wireguard"
		} else {
			rtype = "derp"
		}
		var relayMetadata map[string]interface{}
		if nx.Derper != nil && nx.Derper.certMode == "manual" {
			relayMetadata = map[string]interface{}{"type": rtype, "hostname": nx.Derper.hostname, "certmodemanual": true}
		} else {
			relayMetadata = map[string]interface{}{"type": rtype}
		}

		md, resp, err := nx.client.DevicesApi.UpdateDeviceMetadataKey(context.Background(), deviceId, "relay").Value(relayMetadata).Execute()
		nx.logger.Debugf("Updated relay device %s metadata to: %v", deviceId, md)
		return resp, err
	}
	return nil, nil
}

func (nx *Nexodus) getDeviceRelayMetadata(deviceId string) (client.ModelsDeviceMetadata, *http.Response, error) {
	metadata, resp, err := nx.relayMetadataInformer.Execute()
	if err != nil {
		return client.ModelsDeviceMetadata{}, resp, err
	}
	item, found := metadata[deviceId+"/relay"]
	if !found {
		return client.ModelsDeviceMetadata{}, resp, errors.New("relay metadata not found")
	}
	return item, resp, nil
}
