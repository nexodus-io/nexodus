//go:build !windows

package util

func TimeBeginPeriod(period uint32) error {
	return nil
}

func TimeEndPeriod(period uint32) error {
	return nil
}
