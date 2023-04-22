package nexodus

import (
	"context"
	"errors"
	"fmt"

	"github.com/nexodus-io/nexodus/internal/api/public"
)

func (ax *Nexodus) createOrUpdateDeviceOperation(userID string, endpoints []public.ModelsEndpoint) (public.ModelsDevice, error) {
	d, _, err := ax.client.DevicesApi.CreateDevice(context.Background()).Device(public.ModelsAddDevice{
		UserId:                  userID,
		OrganizationId:          ax.organization,
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
				d, _, err = ax.client.DevicesApi.UpdateDevice(context.Background(), model.Id).Update(public.ModelsUpdateDevice{
					ChildPrefix:             ax.childPrefix,
					EndpointLocalAddressIp4: ax.endpointLocalAddress,
					SymmetricNat:            ax.symmetricNat,
					Hostname:                ax.hostname,
					Endpoints:               endpoints,
				}).Execute()
				if err != nil {
					return public.ModelsDevice{}, fmt.Errorf("error updating device: %w", err)
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
