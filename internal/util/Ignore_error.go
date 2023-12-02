package util

type fnWithErrorResult func() error

// IgnoreError call the passed fn and ignore the errors it returns.  Example `defer util.IgnoreError(file.Close)`
func IgnoreError(fn fnWithErrorResult) {
	_ = fn()
}

func CLose(err *error, fn fnWithErrorResult) {
	e := fn()
	// This is typically, use from a defer to clean up, so avoid stepping on any existing errors
	if e != nil && *err == nil {
		*err = e
	}
}
