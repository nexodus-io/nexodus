package wireguard

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

func defaultTunnelDev(userspaceMode bool) string {
	if userspaceMode {
		return defaultTunnelDevUS()
	}
	return defaultTunnelDevOS()
}

// runCommand runs the cmd and returns the combined stdout and stderr
func runCommand(cmd ...string) (string, error) {
	// #nosec -- G204: Subprocess launched with a potential tainted input or cmd arguments
	output, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run %q: %w (%s)", strings.Join(cmd, " "), err, output)
	}
	return string(output), nil
}

// CreateDirectory create a directory if one does not exist
func CreateDirectory(path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create the directory %s: %w", path, err)
		}
	}
	return nil
}

func fileExists(f string) bool {
	if _, err := os.Stat(f); err != nil {
		return false
	}
	return true
}

// WriteToFile overwrite the contents of a file
func WriteToFile(logger *zap.SugaredLogger, s, file string, filePermissions int) {
	// overwrite the existing file contents
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(filePermissions))
	if err != nil {
		logger.Warnf("Unable to open the file %s to write to: %v", file, err)
	}

	defer func(f *os.File) {
		err = f.Close()
		if err != nil {
			logger.Warnf("Unable to write to file [ %s ] %v", file, err)
		}
	}(f)

	wr := bufio.NewWriter(f)
	_, err = wr.WriteString(s)
	if err != nil {
		logger.Warnf("Unable to write to file [ %s ] %v", file, err)
	}
	if err = wr.Flush(); err != nil {
		logger.Warnf("Unable to write to file [ %s ] %v", file, err)
	}
}
