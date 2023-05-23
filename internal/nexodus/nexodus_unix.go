//go:build linux || darwin

package nexodus

import "errors"

var interfaceErr = errors.New("interface setup error")
