package sort

import (
	"fmt"
	"testing"
)

func TestSort(t *testing.T) {
	fmt.Println(merge([][]string{
		{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L"},
		{"A", "B", "C", "D"},
		{"A", "B", "C", "D"},
		{"A", "B", "C", "D"},
		{"A", "B", "C", "D"},
		{"A", "X", "Z", "D"},
		{"Z", "Y"},
		{"Q"},
	}))
}
