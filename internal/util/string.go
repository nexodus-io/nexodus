package util

func PtrString(v string) *string { return &v }

func FilterOutAllowed(elems []string, allowed map[string]struct{}) (notAllowed []string) {
	for _, e := range elems {
		if _, allow := allowed[e]; !allow {
			notAllowed = append(notAllowed, e)
		}
	}
	return
}
