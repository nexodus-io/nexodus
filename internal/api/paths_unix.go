//go:build !windows

package api

var UnixSocketPath = "/var/run/nexd.sock"
var UnixSocketPathExpression = UnixSocketPath
