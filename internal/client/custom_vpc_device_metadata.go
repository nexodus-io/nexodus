package client

import (
	"github.com/nexodus-io/nexodus/internal/util"
)

// ListInformer creates a *ListInformer which provides a simpler
// API to list devices but which is implemented with the Watch api.  The *ListInformer
// maintains a local device cache which gets updated with the Watch events.
func (r ApiListMetadataInVPCRequest) Informer() *ListInformer[ModelsDeviceMetadata] {
	informer := NewInformer[ModelsDeviceMetadata](&ModelsDeviceMetadataAdaptor{}, r.gtRevision, ApiWatchRequest{
		ctx:        r.ctx,
		ApiService: r.ApiService.client.EventsApi,
	}, map[string]interface{}{
		"vpc-id":   r.id,
		"key":      r.key,
		"prefixes": r.prefix,
	})
	return informer
}

func (r ApiGetDeviceMetadataKeyRequest) Informer() *GetInformer[ModelsDeviceMetadata] {
	informer := NewInformer[ModelsDeviceMetadata](&ModelsDeviceMetadataAdaptor{}, nil, ApiWatchRequest{
		ctx:        r.ctx,
		ApiService: r.ApiService.client.EventsApi,
	}, map[string]interface{}{
		"device-id": r.id,
		"key":       r.key,
	})

	return &GetInformer[ModelsDeviceMetadata]{list: informer}
}

type ModelsDeviceMetadataAdaptor struct{}

func (d ModelsDeviceMetadataAdaptor) Revision(item ModelsDeviceMetadata) int32 {
	return item.GetRevision()
}

func (d ModelsDeviceMetadataAdaptor) Key(item ModelsDeviceMetadata) string {
	return item.GetDeviceId() + "/" + item.GetKey()
}

func (d ModelsDeviceMetadataAdaptor) Kind() string {
	return "device-metadata"
}

func (d ModelsDeviceMetadataAdaptor) Item(value map[string]interface{}) (ModelsDeviceMetadata, error) {
	item := ModelsDeviceMetadata{}
	err := util.JsonUnmarshal(value, &item)
	return item, err
}

var _ InformerAdaptor[ModelsDeviceMetadata] = &ModelsDeviceMetadataAdaptor{}
