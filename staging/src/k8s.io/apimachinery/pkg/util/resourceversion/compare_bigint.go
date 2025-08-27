package resourceversion

import (
	"math/big"
)

func CompareResourceVersionsBigInt(a, b string) (int, error) {
	if !isWellFormed(a) {
		return 0, InvalidResourceVersion{rv: a}
	}
	if !isWellFormed(b) {
		return 0, InvalidResourceVersion{rv: b}
	}
	aInt, ok := big.NewInt(0).SetString(a, 10)
	if !ok {
		return 0, InvalidResourceVersion{rv: a}
	}
	bInt, ok := big.NewInt(0).SetString(b, 10)
	if !ok {
		return 0, InvalidResourceVersion{rv: b}
	}
	return aInt.Cmp(bInt), nil
}
