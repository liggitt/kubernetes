package resourceversion

func CompareResourceVersionsIterate(a, b string) (int, error) {
	// This function is alloc-free, and does complicated things to only do a single-pass on each string (merging format validation and comparison for the equal-length case)

	aLen := len(a)
	bLen := len(b)

	switch {
	case aLen < bLen:
		// if both are well-formed integer strings, shorter is less
		if !isWellFormed(a) {
			return 0, InvalidResourceVersion{rv: a}
		}
		if !isWellFormed(b) {
			return 0, InvalidResourceVersion{rv: b}
		}
		return -1, nil

	case aLen > bLen:
		// if both are well-formed integer strings, longer is more
		if !isWellFormed(a) {
			return 0, InvalidResourceVersion{rv: a}
		}
		if !isWellFormed(b) {
			return 0, InvalidResourceVersion{rv: b}
		}
		return 1, nil

	default:
		// ensure non-zero length, no leading zeros
		if aLen == 0 || a[0] == '0' {
			return 0, InvalidResourceVersion{rv: a}
		}
		if bLen == 0 || b[0] == '0' {
			return 0, InvalidResourceVersion{rv: b}
		}
		// fully iterate, ensuring both are well-formed integer strings, tracking which side is greater
		result := 0
		foundDifference := false
		for i := range a {
			aByte := a[i]
			if !isDigit(aByte) {
				return 0, InvalidResourceVersion{rv: a}
			}
			bByte := b[i]
			if !isDigit(bByte) {
				return 0, InvalidResourceVersion{rv: a}
			}
			if !foundDifference {
				switch {
				case aByte < bByte:
					result = -1
					foundDifference = true
				case aByte > bByte:
					result = 1
					foundDifference = true
				}
			}
		}
		return result, nil
	}
}
