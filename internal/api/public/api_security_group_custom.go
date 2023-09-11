package public

import (
	"github.com/nexodus-io/nexodus/internal/util"
)

// Informer creates a *ApiListSecurityGroupsInformer which provides a simpler
// API to list devices but which is implemented with the Watch api.  The *ApiListSecurityGroupsInformer
// maintains a local device cache which gets updated with the Watch events.
func (r ApiListSecurityGroupsRequest) Informer() *Informer[ModelsSecurityGroup] {
	informer := NewInformer[ModelsSecurityGroup](&SecurityGroupAdaptor{}, r.gtRevision, ApiWatchEventsRequest{
		ctx:            r.ctx,
		ApiService:     r.ApiService.client.OrganizationsApi,
		organizationId: r.organizationId,
	})
	return informer
}

type SecurityGroupAdaptor struct{}

func (d SecurityGroupAdaptor) Revision(item ModelsSecurityGroup) int32 {
	return item.Revision
}

func (d SecurityGroupAdaptor) Key(item ModelsSecurityGroup) string {
	return item.Id
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
