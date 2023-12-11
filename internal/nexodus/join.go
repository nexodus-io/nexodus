package nexodus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/nexodus-io/nexodus/internal/api/public"
)

func (nx *Nexodus) createOrUpdateDeviceOperation(userID string, endpoints []public.ModelsEndpoint) (public.ModelsDevice, string, error) {
	newDev := public.ModelsAddDevice{
		VpcId:           nx.vpc.Id,
		SecurityGroupId: nx.securityGroupId,
		PublicKey:       nx.wireguardPubKey,
		AdvertiseCidrs:  nx.advertiseCidrs,
		SymmetricNat:    nx.symmetricNat,
		Hostname:        nx.hostname,
		Relay:           nx.relay || nx.relayDerp,
		Os:              nx.os,
		Endpoints:       endpoints,
	}

	if len(nx.requestedIP) > 0 {
		newDev.Ipv4TunnelIps = []public.ModelsTunnelIP{
			{
				Address: nx.requestedIP,
				Cidr:    nx.vpc.Ipv4Cidr,
			},
		}
	}
	d, _, err := nx.client.DevicesApi.CreateDevice(context.Background()).Device(newDev).Execute()
	deviceOperationMsg := "Successfully registered device"
	var resp *http.Response
	if err != nil {
		var apiError *public.GenericOpenAPIError
		if errors.As(err, &apiError) {
			switch model := apiError.Model().(type) {
			case public.ModelsConflictsError:
				d, resp, err = nx.client.DevicesApi.UpdateDevice(context.Background(), model.Id).Update(public.ModelsUpdateDevice{
					VpcId:          nx.vpc.Id,
					AdvertiseCidrs: nx.advertiseCidrs,
					SymmetricNat:   nx.symmetricNat,
					Hostname:       nx.hostname,
					Endpoints:      endpoints,
					Relay:          nx.relay,
				}).Execute()
				deviceOperationMsg = "Reconnected as device"
				if err != nil {
					respText := ""
					if resp != nil {
						bytes, err := io.ReadAll(resp.Body)
						if err != nil {
							return public.ModelsDevice{}, "", fmt.Errorf("error updating device: %w - %s", err, resp.Status)
						}
						respText = string(bytes)
					}
					return public.ModelsDevice{}, "", fmt.Errorf("error updating device: %w - %s", err, respText)
				}
			default:
				return public.ModelsDevice{}, "", fmt.Errorf("error creating device: %w", err)
			}
		} else {
			return public.ModelsDevice{}, "", fmt.Errorf("error creating device: %w", err)
		}
	}

	resp, err = nx.updateDeviceRelayMetadata(d.Id)
	if err != nil {
		respText := ""
		if resp != nil {
			bytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return public.ModelsDevice{}, "", fmt.Errorf("error updating device metadata: %w - %s", err, resp.Status)
			}
			respText = string(bytes)
		}
		return public.ModelsDevice{}, "", fmt.Errorf("error updating device metadata: %w - %s", err, respText)
	}

	return *d, deviceOperationMsg, nil
}

func (nx *Nexodus) updateDeviceRelayMetadata(deviceId string)  (*http.Response, error){
	if nx.relay || nx.relayDerp {
		var rtype interface{}
		if nx.relay {
			rtype = "wireguard"
		} else {
			rtype = "derp"
		}

		relaytype := map[string]interface{}{"type": rtype}
		_, resp, err := nx.client.DevicesApi.UpdateDeviceMetadataKey(context.Background(), deviceId, "relay").Value(relaytype).Execute()
		nx.logger.Debugf("Updated device metadata: %s", resp.Status)
		return resp, err
	}
	return nil, nil
}

func (nx *Nexodus) getDeviceRelayMetadata(deviceId string) (public.ModelsDeviceMetadata, *http.Response, error) {
	metadata, resp, err := nx.client.DevicesApi.GetDeviceMetadataKey(context.Background(), deviceId, "relay").Execute()
	if err != nil {
		return public.ModelsDeviceMetadata{}, resp, err
	}
	return *metadata, resp, nil
}
