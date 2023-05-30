package nexodus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/nexodus-io/nexodus/internal/api/public"
)

func (ax *Nexodus) createOrUpdateDeviceOperation(userID string, endpoints []public.ModelsEndpoint) (public.ModelsDevice, error) {
	d, _, err := ax.client.DevicesApi.CreateDevice(context.Background()).Device(public.ModelsAddDevice{
		UserId:                  userID,
		OrganizationId:          ax.org.Id,
		PublicKey:               ax.wireguardPubKey,
		TunnelIp:                ax.requestedIP,
		ChildPrefix:             ax.childPrefix,
		EndpointLocalAddressIp4: ax.endpointLocalAddress,
		SymmetricNat:            ax.symmetricNat,
		Hostname:                ax.hostname,
		Relay:                   ax.relay,
		Os:                      ax.os,
		Endpoints:               endpoints,
	}).Execute()

	if err != nil {
		var apiError *public.GenericOpenAPIError
		if errors.As(err, &apiError) {
			switch model := apiError.Model().(type) {
			case public.ModelsConflictsError:
				var resp *http.Response
				d, resp, err = ax.client.DevicesApi.UpdateDevice(context.Background(), model.Id).Update(public.ModelsUpdateDevice{
					ChildPrefix:             ax.childPrefix,
					EndpointLocalAddressIp4: ax.endpointLocalAddress,
					SymmetricNat:            ax.symmetricNat,
					Hostname:                ax.hostname,
					Endpoints:               endpoints,
					OrganizationId:          ax.org.Id,
				}).Execute()
				if err != nil {
					respText := ""
					if resp != nil {
						bytes, err := io.ReadAll(resp.Body)
						if err != nil {
							return public.ModelsDevice{}, fmt.Errorf("error updating device: %w - %s", err, resp.Status)
						}
						respText = string(bytes)
					}
					return public.ModelsDevice{}, fmt.Errorf("error updating device: %w - %s", err, respText)
				}
			default:
				return public.ModelsDevice{}, fmt.Errorf("error creating device: %w", err)
			}
		} else {
			return public.ModelsDevice{}, fmt.Errorf("error creating device: %w", err)
		}
	}

	return *d, nil
}
