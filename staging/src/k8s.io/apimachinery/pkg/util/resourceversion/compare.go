package resourceversion

import (
	"fmt"
	"strings"
)

type InvalidResourceVersion struct {
	rv string
}

func (i InvalidResourceVersion) Error() string {
	return fmt.Sprintf("invalid resourceVersion: %s", i.rv)
}

// CompareResourceVersions returns an integer comparing two resourceVersions.
// The result will be 0 if a == b, -1 if a < b, and +1 if a > b.
// If either resource version is not well-formed, an error is returned.
//
// Well-formed comparable resourceVersions must:
//   - be non-zero length
//   - only contain digits 0-9
//   - start with a non-zero digit
func CompareResourceVersions(a, b string) (int, error) {
	if !isWellFormed(a) {
		return 0, InvalidResourceVersion{rv: a}
	}
	if !isWellFormed(b) {
		return 0, InvalidResourceVersion{rv: b}
	}
	// both are well-formed integer strings with no leading zeros
	aLen := len(a)
	bLen := len(b)
	switch {
	case aLen < bLen:
		// shorter is less
		return -1, nil
	case aLen > bLen:
		// longer is greater
		return 1, nil
	default:
		// equal-length compares lexically
		return strings.Compare(a, b), nil
	}
}

func isWellFormed(s string) bool {
	if len(s) == 0 {
		return false
	}
	if s[0] == '0' {
		return false
	}
	for i := range s {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
