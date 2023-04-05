package util

import (
	"net"
)

/* maybe we can use a generic version in the future...
func ToStringSlice[S fmt.Stringer](items []S) (result []string) {
*/

func IPNetSliceToStringSlice(items []net.IPNet) (result []string) {
	for _, i := range items {
		result = append(result, i.String())
	}
	return
}
