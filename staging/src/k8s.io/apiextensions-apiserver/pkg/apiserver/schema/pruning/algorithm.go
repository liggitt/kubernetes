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
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
)

// Prune removes object fields in obj which are not specified in s. It skips TypeMeta and ObjectMeta fields
// if XEmbeddedResource is set to true, or for the root if isResourceRoot=true, i.e. it does not
// prune unknown metadata fields.
// It returns the set of fields that it prunes.
func Prune(obj interface{}, s *structuralschema.Structural, isResourceRoot bool) map[string]bool {
	if isResourceRoot {
		if s == nil {
			s = &structuralschema.Structural{}
		}
		if !s.XEmbeddedResource {
			clone := *s
			clone.XEmbeddedResource = true
			s = &clone
		}
	}
	return prune(obj, s)
}

var metaFields = map[string]bool{
	"apiVersion": true,
	"kind":       true,
	"metadata":   true,
}

func prune(x interface{}, s *structuralschema.Structural) map[string]bool {
	if s != nil && s.XPreserveUnknownFields {
		return skipPrune(x, s)
	}

	pruning := map[string]bool{}
	switch x := x.(type) {
	case map[string]interface{}:
		if s == nil {
			for k := range x {
				if !metaFields[k] {
					pruning[k] = true
				}
				delete(x, k)
			}
			return pruning
		}
		for k, v := range x {
			if s.XEmbeddedResource && metaFields[k] {
				continue
			}
			prop, ok := s.Properties[k]
			if ok {
				pruned := prune(v, &prop)
				for k, b := range pruned {
					pruning[k] = b
				}
			} else if s.AdditionalProperties != nil {
				pruned := prune(v, s.AdditionalProperties.Structural)
				for k, b := range pruned {
					pruning[k] = b
				}
			} else {
				if !metaFields[k] {
					pruning[k] = true
				}
				delete(x, k)
			}
		}
	case []interface{}:
		if s == nil {
			for _, v := range x {
				pruned := prune(v, nil)
				for k, b := range pruned {
					pruning[k] = b
				}
			}
			return pruning
		}
		for _, v := range x {
			pruned := prune(v, s.Items)
			for k, b := range pruned {
				pruning[k] = b
			}
		}
	default:
		// scalars, do nothing
	}
	return pruning
}

func skipPrune(x interface{}, s *structuralschema.Structural) map[string]bool {
	pruning := map[string]bool{}
	if s == nil {
		return pruning
	}

	switch x := x.(type) {
	case map[string]interface{}:
		for k, v := range x {
			if s.XEmbeddedResource && metaFields[k] {
				continue
			}
			if prop, ok := s.Properties[k]; ok {
				pruned := prune(v, &prop)
				for k, b := range pruned {
					pruning[k] = b
				}
			} else if s.AdditionalProperties != nil {
				pruned := prune(v, s.AdditionalProperties.Structural)
				for k, b := range pruned {
					pruning[k] = b
				}
			}
		}
	case []interface{}:
		for _, v := range x {
			skipPruned := skipPrune(v, s.Items)
			for k, b := range skipPruned {
				pruning[k] = b
			}
		}
	default:
		// scalars, do nothing
	}
	return pruning
}
