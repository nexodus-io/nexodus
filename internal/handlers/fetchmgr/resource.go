package fetchmgr

import "gorm.io/gorm"

type ResourceList interface {
	Len() int
	Item(i int) (any, string, uint64, gorm.DeletedAt)
}

type ResourceItem struct {
	Item      any            `json:"item,omitempty"`
	Id        string         `json:"id,omitempty"`
	Revision  uint64         `json:"revision,omitempty"`
	DeletedAt gorm.DeletedAt `json:"deleted_at"`
}
type ResourceItemList []ResourceItem

func (l ResourceItemList) Item(i int) (any, string, uint64, gorm.DeletedAt) {
	item := l[i]
	return item.Item, item.Id, item.Revision, item.DeletedAt
}

func (l ResourceItemList) Len() int {
	return len(l)
}
