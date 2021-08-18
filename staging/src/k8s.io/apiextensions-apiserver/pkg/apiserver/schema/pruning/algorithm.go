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
	"fmt"
	"strconv"

	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
)

// PruneOptions sets options for pruning
// unknown fields
type PruneOptions struct {
	// IsResourceRoot indicates whether
	// this is the root of the object.
	IsResourceRoot bool
	// PruneOnPreserveUnknownFields
	// indicates whether Prune should
	// prune fields that are unknown
	// even if s.XPreserveUnknownFields
	// is set.
	PruneOnPreserveUnknownFields bool
	// parentPath collects the path that the pruning
	// takes as it travers the object.
	// It is used to report the full path to any unknown
	// fields that the pruning encounters.
	parentPath string
}

// Prune removes object fields in obj which are not specified in s. It skips TypeMeta and ObjectMeta fields
// if XEmbeddedResource is set to true, or for the root if isResourceRoot=true, i.e. it does not
// prune unknown metadata fields.
// It returns the set of fields that it prunes.
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
	pruned := prune(obj, s, opts)
	return pruned
}

// Prune calls into PruneWithOptions
func Prune(obj interface{}, s *structuralschema.Structural, isResourceRoot bool) []string {
	return PruneWithOptions(obj, s, PruneOptions{true, false, ""})
}

var metaFields = map[string]bool{
	"apiVersion": true,
	"kind":       true,
	"metadata":   true,
}

func copyOptionsUpdatePath(opts PruneOptions, parent string) PruneOptions {
	newPath := fmt.Sprintf("%s/%s", opts.parentPath, parent)
	return PruneOptions{
		PruneOnPreserveUnknownFields: opts.PruneOnPreserveUnknownFields,
		parentPath:                   newPath,
	}
}

func prune(x interface{}, s *structuralschema.Structural, opts PruneOptions) []string {
	if s != nil && s.XPreserveUnknownFields && !opts.PruneOnPreserveUnknownFields {
		return skipPrune(x, s, PruneOptions{
			parentPath: opts.parentPath,
		})
	}

	var pruned []string
	switch x := x.(type) {
	case map[string]interface{}:
		if s == nil {
			for k := range x {
				if !metaFields[k] {
					pruned = append(pruned, fmt.Sprintf("%s/%s", opts.parentPath, k))
				}
				delete(x, k)
			}
			return pruned
		}
		for k, v := range x {
			if s.XEmbeddedResource && metaFields[k] {
				continue
			}
			prop, ok := s.Properties[k]
			if ok {
				pruned = append(pruned, prune(v, &prop, copyOptionsUpdatePath(opts, k))...)
			} else if s.AdditionalProperties != nil {
				pruned = append(pruned, prune(v, s.AdditionalProperties.Structural, copyOptionsUpdatePath(opts, k))...)
			} else {
				if !metaFields[k] {
					pruned = append(pruned, fmt.Sprintf("%s/%s", opts.parentPath, k))
				}
				delete(x, k)
			}
		}
	case []interface{}:
		if s == nil {
			for i, v := range x {
				pruned = append(pruned, prune(v, nil, copyOptionsUpdatePath(opts, strconv.Itoa(i)))...)
			}
			return pruned
		}
		for i, v := range x {
			pruned = append(pruned, prune(v, s.Items, copyOptionsUpdatePath(opts, strconv.Itoa(i)))...)
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

	switch x := x.(type) {
	case map[string]interface{}:
		for k, v := range x {
			if s.XEmbeddedResource && metaFields[k] {
				continue
			}
			if prop, ok := s.Properties[k]; ok {
				pruned = append(pruned, prune(v, &prop, PruneOptions{
					parentPath: fmt.Sprintf("%s/%s", opts.parentPath, k),
				})...)
			} else if s.AdditionalProperties != nil {
				pruned = append(pruned, prune(v, s.AdditionalProperties.Structural, PruneOptions{
					parentPath: fmt.Sprintf("%s/%s", opts.parentPath, k),
				})...)
			}
		}
	case []interface{}:
		for i, v := range x {
			pruned = append(pruned, skipPrune(v, s.Items, PruneOptions{
				parentPath: fmt.Sprintf("%s/%d", opts.parentPath, i),
			})...)
		}
	default:
		// scalars, do nothing
	}
	return pruned
}
