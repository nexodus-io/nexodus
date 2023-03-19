package util

// IgnoreError is a noop.  It is used to eat errors.  Example `defer util.IgnoreError(file.Close())`
func IgnoreError(err error) {
}
