/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pruning

import (
	"strconv"
	"strings"

	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
)

// PruneOptions sets options for pruning
// unknown fields
type PruneOptions struct {
	// IsResourceRoot indicates whether
	// this is the root of the object.
	IsResourceRoot bool
	// parentPath collects the path that the pruning
	// takes as it travers the object.
	// It is used to report the full path to any unknown
	// fields that the pruning encounters.
	parentPath []string
	// ReturnPruned defines whether we want to track the
	// fields that are pruned
	ReturnPruned bool
}

// PruneWithOptions removes object fields in obj which are not specified in s. It skips TypeMeta
// and ObjectMeta fields if XEmbeddedResource is set to true, or for the root if isResourceRoot=true,
// i.e. it does not prune unknown metadata fields.
// It returns the set of fields that it prunes if opts.ReturnPruned is true
func PruneWithOptions(obj interface{}, s *structuralschema.Structural, opts PruneOptions) []string {
	if opts.IsResourceRoot {
		if s == nil {
			s = &structuralschema.Structural{}
		}
		if !s.XEmbeddedResource {
			clone := *s
			clone.XEmbeddedResource = true
			s = &clone
		}
	}
	return prune(obj, s, opts)
}

// Prune is equivalent to
// PruneWithOptions(obj, s, PruneOptions{IsResourceRoot: isResourceRoot})
func Prune(obj interface{}, s *structuralschema.Structural, isResourceRoot bool) {
	PruneWithOptions(obj, s, PruneOptions{isResourceRoot, []string{}, false})
}

var metaFields = map[string]bool{
	"apiVersion": true,
	"kind":       true,
	"metadata":   true,
}

func appendKey(path []string, key string, returnPruned bool) []string {
	if !returnPruned {
		return path
	}
	if len(path) > 0 {
		path = append(path, ".")
	}
	return append(path, key)
}

func appendIndex(path []string, index int, returnPruned bool) []string {
	if !returnPruned {
		return path
	}
	return append(path, "[", strconv.Itoa(index), "]")
}

func appendIfPruned(returnPruned bool, s []string, vs ...string) []string {
	if !returnPruned {
		return s
	}
	return append(s, vs...)
}

func prune(x interface{}, s *structuralschema.Structural, opts PruneOptions) []string {
	if s != nil && s.XPreserveUnknownFields {
		return skipPrune(x, s, PruneOptions{
			parentPath:   opts.parentPath,
			ReturnPruned: opts.ReturnPruned,
		})
	}

	var pruned []string
	origPathLen := len(opts.parentPath)
	switch x := x.(type) {
	case map[string]interface{}:
		if s == nil {
			for k := range x {
				pruned = appendIfPruned(opts.ReturnPruned, pruned, strings.Join(appendKey(opts.parentPath, k, opts.ReturnPruned), ""))
				delete(x, k)
				opts.parentPath = opts.parentPath[:origPathLen]
			}
			return pruned
		}
		for k, v := range x {
			if s.XEmbeddedResource && metaFields[k] {
				continue
			}
			opts.parentPath = appendKey(opts.parentPath, k, opts.ReturnPruned)
			prop, ok := s.Properties[k]
			if ok {
				pruned = appendIfPruned(opts.ReturnPruned, pruned, prune(v, &prop, opts)...)
			} else if s.AdditionalProperties != nil {
				pruned = appendIfPruned(opts.ReturnPruned, pruned, prune(v, s.AdditionalProperties.Structural, opts)...)
			} else {
				if !metaFields[k] || len(opts.parentPath) > 1 {
					pruned = appendIfPruned(opts.ReturnPruned, pruned, strings.Join(opts.parentPath, ""))
				}
				delete(x, k)
			}
			opts.parentPath = opts.parentPath[:origPathLen]
		}
	case []interface{}:
		if s == nil {
			for i, v := range x {
				opts.parentPath = appendIndex(opts.parentPath, i, opts.ReturnPruned)
				pruned = appendIfPruned(opts.ReturnPruned, pruned, prune(v, nil, opts)...)
				opts.parentPath = opts.parentPath[:origPathLen]
			}
			return pruned
		}
		for i, v := range x {
			opts.parentPath = appendIndex(opts.parentPath, i, opts.ReturnPruned)
			pruned = appendIfPruned(opts.ReturnPruned, pruned, prune(v, s.Items, opts)...)
			opts.parentPath = opts.parentPath[:origPathLen]
		}
	default:
		// scalars, do nothing
	}
	return pruned
}

func skipPrune(x interface{}, s *structuralschema.Structural, opts PruneOptions) []string {
	var pruned []string
	if s == nil {
		return pruned
	}
	origPathLen := len(opts.parentPath)

	switch x := x.(type) {
	case map[string]interface{}:
		for k, v := range x {
			if s.XEmbeddedResource && metaFields[k] {
				continue
			}
			opts.parentPath = appendKey(opts.parentPath, k, opts.ReturnPruned)
			if prop, ok := s.Properties[k]; ok {
				pruned = appendIfPruned(opts.ReturnPruned, pruned, prune(v, &prop, opts)...)
			} else if s.AdditionalProperties != nil {
				pruned = appendIfPruned(opts.ReturnPruned, pruned, prune(v, s.AdditionalProperties.Structural, opts)...)
			}
			opts.parentPath = opts.parentPath[:origPathLen]
		}
	case []interface{}:
		for i, v := range x {
			opts.parentPath = appendIndex(opts.parentPath, i, opts.ReturnPruned)
			pruned = appendIfPruned(opts.ReturnPruned, pruned, prune(v, s.Items, opts)...)
			opts.parentPath = opts.parentPath[:origPathLen]
		}
	default:
		// scalars, do nothing
	}
	return pruned
}
