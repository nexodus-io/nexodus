package util

import (
	"net"
	"strconv"
	"strings"
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

func StringToInt64(s string) int64 {
	var result int64
	result, _ = strconv.ParseInt(s, 10, 64)
	return result
}

func SplitKeyValue(s string) (result []string) {
	i := strings.Index(s, "=")
	if i == -1 {
		return
	}
	result = append(result, s[:i])
	result = append(result, s[i+1:])
	return
}
