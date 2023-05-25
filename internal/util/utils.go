package util

import (
	"net"
	"strconv"
	"strings"
	"time"
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

// ParseTime attempts to parse a time string in three possible formats
// to handle UTC and local time differences between Darwin and Linux
// and the proxy mode. On each system we get different time formats
// for the last wireguard handshake time.
func ParseTime(timeStr string) (time.Time, error) {
	var t time.Time
	var ut int64
	var err error
	if t, err = time.Parse(time.RFC3339Nano, timeStr); err == nil {
		return t.UTC(), nil
	}
	if t, err = time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", timeStr); err == nil {
		return t.UTC(), nil
	}
	if ut, err = strconv.ParseInt(timeStr, 10, 64); err == nil {
		if ut != 0 {
			t = time.Unix(ut, 0)
		}
	}
	return t.UTC(), err
}
