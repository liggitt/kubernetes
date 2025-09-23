package sort

import (
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/util/sets"
)

func merge(peers [][]string) []string {
	items := []string{}
	lessThan := map[string]sets.Set[string]{}
	// build the unsorted list of items, and populate the graph with all the immediate "less than" relationships
	for _, peerItems := range peers {
		for i, lhs := range peerItems {
			if _, seen := lessThan[lhs]; !seen {
				items = append(items, lhs)
				lessThan[lhs] = sets.New[string]()
			}
			if i < len(peerItems)-1 {
				lessThan[lhs].Insert(peerItems[i+1])
			}
		}
	}

	// shortcut if one of the peers has all the items already
	for _, peerItems := range peers {
		if len(peerItems) == len(items) && sets.New(peerItems...).Len() == len(items) {
			copy(items, peerItems)
			return items
		}
	}

	// debug print the graph
	for _, item := range items {
		fmt.Println(item, sets.List(lessThan[item]))
	}

	// sort based on finding paths between pairs
	sort.Slice(items, func(i, j int) bool {
		itemI := items[i]
		itemJ := items[j]
		if pathFrom(itemI, itemJ, lessThan) {
			return true
		}
		if pathFrom(itemJ, itemI, lessThan) {
			return false
		}
		// if there's no path, sort lexically
		return itemI < itemJ
	})

	return items
}

func pathFrom(from, to string, links map[string]sets.Set[string]) bool {
	visited := sets.New[string]()
	tovisit := sets.New[string](from)
	for {
		v, ok := tovisit.PopAny()
		if !ok {
			return false
		}
		visited.Insert(v)
		for next := range links[v] {
			if next == to {
				return true
			}
			if !visited.Has(next) {
				tovisit.Insert(next)
			}
		}
	}
}
