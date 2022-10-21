//go:build darwin

package apex

// routeExists currently only used for darwin build purposes
func routeExists(s string) bool {
	return false
}
