package client

// PtrOptionalString is a helper routine that returns a pointer to given string value if it's not a zero value.
func PtrOptionalString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
