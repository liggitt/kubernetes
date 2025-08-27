package resourceversion

func CompareResourceVersionsWithGTLT(a, b string) (int, error) {
	// if both are well-formed integer strings, shorter is less
	if !isWellFormed(a) {
		return 0, InvalidResourceVersion{rv: a}
	}
	if !isWellFormed(b) {
		return 0, InvalidResourceVersion{rv: b}
	}
	aLen := len(a)
	bLen := len(b)
	switch {
	case aLen < bLen:
		return -1, nil
	case aLen > bLen:
		return 1, nil
	default:
		if a < b {
			return -1, nil
		}
		if b > a {
			return 1, nil
		}
		return 0, nil
	}
}
