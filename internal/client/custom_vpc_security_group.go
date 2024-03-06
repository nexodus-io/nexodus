package client

import (
	"github.com/nexodus-io/nexodus/internal/util"
)

// Informer creates a *Informer which provides a simpler
// API to list devices but which is implemented with the Watch api.  The *Informer
// maintains a local device cache which gets updated with the Watch events.
func (r ApiListSecurityGroupsInVPCRequest) Informer() *Informer[ModelsSecurityGroup] {
	informer := NewInformer[ModelsSecurityGroup](&SecurityGroupAdaptor{}, r.gtRevision, ApiWatchRequest{
		ctx:        r.ctx,
		ApiService: r.ApiService.client.EventsApi,
	}, map[string]interface{}{
		"vpc-id": r.id,
	})
	return informer
}

type SecurityGroupAdaptor struct{}

func (d SecurityGroupAdaptor) Revision(item ModelsSecurityGroup) int32 {
	return item.GetRevision()
}

func (d SecurityGroupAdaptor) Key(item ModelsSecurityGroup) string {
	return item.GetId()
}

func (d SecurityGroupAdaptor) Kind() string {
	return "security-group"
}

func (d SecurityGroupAdaptor) Item(value map[string]interface{}) (ModelsSecurityGroup, error) {
	item := ModelsSecurityGroup{}
	err := util.JsonUnmarshal(value, &item)
	return item, err
}

var _ InformerAdaptor[ModelsSecurityGroup] = &SecurityGroupAdaptor{}
