package client

import (
	"github.com/nexodus-io/nexodus/internal/util"
)

// ListInformer creates a *ListInformer which provides a simpler
// API to list devices but which is implemented with the Watch api.  The *ListInformer
// maintains a local device cache which gets updated with the Watch events.
func (r ApiListDevicesInVPCRequest) Informer() *ListInformer[ModelsDevice] {
	informer := NewInformer[ModelsDevice](&DeviceAdaptor{}, r.gtRevision, ApiWatchRequest{
		ctx:        r.ctx,
		ApiService: r.ApiService.client.EventsApi,
	}, map[string]interface{}{
		"vpc-id": r.id,
	})
	return informer
}

type DeviceAdaptor struct{}

func (d DeviceAdaptor) Revision(item ModelsDevice) int32 {
	return item.GetRevision()
}

func (d DeviceAdaptor) Key(item ModelsDevice) string {
	return item.GetId()
}

func (d DeviceAdaptor) Kind() string {
	return "device"
}

func (d DeviceAdaptor) Item(value map[string]interface{}) (ModelsDevice, error) {
	item := ModelsDevice{}
	err := util.JsonUnmarshal(value, &item)
	return item, err
}

var _ InformerAdaptor[ModelsDevice] = &DeviceAdaptor{}
