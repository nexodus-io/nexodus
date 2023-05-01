package util

type fnWithErrorResult func() error

// IgnoreError call the passed fn and ignore the errors it returns.  Example `defer util.IgnoreError(file.Close)`
func IgnoreError(fn fnWithErrorResult) {
	_ = fn()
}
