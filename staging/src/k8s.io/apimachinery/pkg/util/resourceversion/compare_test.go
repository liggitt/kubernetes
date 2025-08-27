package resourceversion

import (
	"regexp"
	"testing"
)

func BenchmarkIsIntegerString(b *testing.B) {
	simpleIntegerPattern := regexp.MustCompile(`^[1-9][0-9]+$`)
	rv := "340282366920938463463374607431768211455"
	b.Run("regexp", func(b *testing.B) {
		for b.Loop() {
			if !simpleIntegerPattern.MatchString(rv) {
				b.Fail()
			}
		}
	})

	b.Run("iterate", func(b *testing.B) {
		for b.Loop() {
			if !isWellFormed(rv) {
				b.Fail()
			}
		}
	})
}

func BenchmarkCompare(b *testing.B) {
	rv1 := "340282366920938463463374607431768211455" // max 128 - 1
	rv2 := "340282366920938463463374607431768211456" // max 128
	b.Run("bigInt", func(b *testing.B) {
		for b.Loop() {
			result, err := CompareResourceVersionsBigInt(rv1, rv2)
			if err != nil {
				b.Fatal()
			}
			if result != -1 {
				b.Fatal()
			}
		}
	})

	b.Run("string-iterate", func(b *testing.B) {
		for b.Loop() {
			result, err := CompareResourceVersionsIterate(rv1, rv2)
			if err != nil {
				b.Fatal()
			}
			if result != -1 {
				b.Fatal()
			}
		}
	})

	b.Run("string-compare", func(b *testing.B) {
		for b.Loop() {
			result, err := CompareResourceVersions(rv1, rv2)
			if err != nil {
				b.Fatal()
			}
			if result != -1 {
				b.Fatal()
			}
		}
	})

	b.Run("string-<>", func(b *testing.B) {
		for b.Loop() {
			result, err := CompareResourceVersionsWithGTLT(rv1, rv2)
			if err != nil {
				b.Fatal()
			}
			if result != -1 {
				b.Fatal()
			}
		}
	})
}
