package testing

// ErrorMessageEquals compare if two error message is equal.
// 1. nil error == nil error
// 2. nil error != non nil error
// 3. Other wise compare error.Error() returned string
func ErrorMessageEquals(a, b error) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	return a.Error() == b.Error()
}
