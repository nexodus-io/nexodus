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
	d, _, err := nx.client.DevicesApi.CreateDevice(context.Background()).Device(public.ModelsAddDevice{
		VpcId:     nx.vpc.Id,
		PublicKey: nx.wireguardPubKey,
		Ipv4TunnelIps: []public.ModelsTunnelIP{
			{
				Address: nx.requestedIP,
				Cidr:    nx.vpc.Ipv4Cidr,
			},
		},

		AdvertiseCidrs: nx.advertiseCidrs,
		SymmetricNat:   nx.symmetricNat,
		Hostname:       nx.hostname,
		Relay:          nx.relay,
		Os:             nx.os,
		Endpoints:      endpoints,
	}).Execute()
	deviceOperationMsg := "Successfully registered device"
	if err != nil {
		var apiError *public.GenericOpenAPIError
		if errors.As(err, &apiError) {
			switch model := apiError.Model().(type) {
			case public.ModelsConflictsError:
				var resp *http.Response
				d, resp, err = nx.client.DevicesApi.UpdateDevice(context.Background(), model.Id).Update(public.ModelsUpdateDevice{
					AdvertiseCidrs: nx.advertiseCidrs,
					SymmetricNat:   nx.symmetricNat,
					Hostname:       nx.hostname,
					Endpoints:      endpoints,
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

	return *d, deviceOperationMsg, nil
}
