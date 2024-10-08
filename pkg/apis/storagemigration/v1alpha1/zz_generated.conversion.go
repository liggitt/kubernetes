//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright The Kubernetes Authors.

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

// Code generated by conversion-gen. DO NOT EDIT.

package v1alpha1

import (
	unsafe "unsafe"

	v1 "k8s.io/api/core/v1"
	storagemigrationv1alpha1 "k8s.io/api/storagemigration/v1alpha1"
	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
	storagemigration "k8s.io/kubernetes/pkg/apis/storagemigration"
)

func init() {
	localSchemeBuilder.Register(RegisterConversions)
}

// RegisterConversions adds conversion functions to the given scheme.
// Public to allow building arbitrary schemes.
func RegisterConversions(s *runtime.Scheme) error {
	if err := s.AddGeneratedConversionFunc((*storagemigrationv1alpha1.GroupVersionResource)(nil), (*storagemigration.GroupVersionResource)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_GroupVersionResource_To_storagemigration_GroupVersionResource(a.(*storagemigrationv1alpha1.GroupVersionResource), b.(*storagemigration.GroupVersionResource), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*storagemigration.GroupVersionResource)(nil), (*storagemigrationv1alpha1.GroupVersionResource)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_storagemigration_GroupVersionResource_To_v1alpha1_GroupVersionResource(a.(*storagemigration.GroupVersionResource), b.(*storagemigrationv1alpha1.GroupVersionResource), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*storagemigrationv1alpha1.MigrationCondition)(nil), (*storagemigration.MigrationCondition)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_MigrationCondition_To_storagemigration_MigrationCondition(a.(*storagemigrationv1alpha1.MigrationCondition), b.(*storagemigration.MigrationCondition), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*storagemigration.MigrationCondition)(nil), (*storagemigrationv1alpha1.MigrationCondition)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_storagemigration_MigrationCondition_To_v1alpha1_MigrationCondition(a.(*storagemigration.MigrationCondition), b.(*storagemigrationv1alpha1.MigrationCondition), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*storagemigrationv1alpha1.StorageVersionMigration)(nil), (*storagemigration.StorageVersionMigration)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_StorageVersionMigration_To_storagemigration_StorageVersionMigration(a.(*storagemigrationv1alpha1.StorageVersionMigration), b.(*storagemigration.StorageVersionMigration), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*storagemigration.StorageVersionMigration)(nil), (*storagemigrationv1alpha1.StorageVersionMigration)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_storagemigration_StorageVersionMigration_To_v1alpha1_StorageVersionMigration(a.(*storagemigration.StorageVersionMigration), b.(*storagemigrationv1alpha1.StorageVersionMigration), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*storagemigrationv1alpha1.StorageVersionMigrationList)(nil), (*storagemigration.StorageVersionMigrationList)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_StorageVersionMigrationList_To_storagemigration_StorageVersionMigrationList(a.(*storagemigrationv1alpha1.StorageVersionMigrationList), b.(*storagemigration.StorageVersionMigrationList), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*storagemigration.StorageVersionMigrationList)(nil), (*storagemigrationv1alpha1.StorageVersionMigrationList)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_storagemigration_StorageVersionMigrationList_To_v1alpha1_StorageVersionMigrationList(a.(*storagemigration.StorageVersionMigrationList), b.(*storagemigrationv1alpha1.StorageVersionMigrationList), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*storagemigrationv1alpha1.StorageVersionMigrationSpec)(nil), (*storagemigration.StorageVersionMigrationSpec)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_StorageVersionMigrationSpec_To_storagemigration_StorageVersionMigrationSpec(a.(*storagemigrationv1alpha1.StorageVersionMigrationSpec), b.(*storagemigration.StorageVersionMigrationSpec), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*storagemigration.StorageVersionMigrationSpec)(nil), (*storagemigrationv1alpha1.StorageVersionMigrationSpec)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_storagemigration_StorageVersionMigrationSpec_To_v1alpha1_StorageVersionMigrationSpec(a.(*storagemigration.StorageVersionMigrationSpec), b.(*storagemigrationv1alpha1.StorageVersionMigrationSpec), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*storagemigrationv1alpha1.StorageVersionMigrationStatus)(nil), (*storagemigration.StorageVersionMigrationStatus)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_StorageVersionMigrationStatus_To_storagemigration_StorageVersionMigrationStatus(a.(*storagemigrationv1alpha1.StorageVersionMigrationStatus), b.(*storagemigration.StorageVersionMigrationStatus), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*storagemigration.StorageVersionMigrationStatus)(nil), (*storagemigrationv1alpha1.StorageVersionMigrationStatus)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_storagemigration_StorageVersionMigrationStatus_To_v1alpha1_StorageVersionMigrationStatus(a.(*storagemigration.StorageVersionMigrationStatus), b.(*storagemigrationv1alpha1.StorageVersionMigrationStatus), scope)
	}); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1alpha1_GroupVersionResource_To_storagemigration_GroupVersionResource(in *storagemigrationv1alpha1.GroupVersionResource, out *storagemigration.GroupVersionResource, s conversion.Scope) error {
	out.Group = in.Group
	out.Version = in.Version
	out.Resource = in.Resource
	return nil
}

// Convert_v1alpha1_GroupVersionResource_To_storagemigration_GroupVersionResource is an autogenerated conversion function.
func Convert_v1alpha1_GroupVersionResource_To_storagemigration_GroupVersionResource(in *storagemigrationv1alpha1.GroupVersionResource, out *storagemigration.GroupVersionResource, s conversion.Scope) error {
	return autoConvert_v1alpha1_GroupVersionResource_To_storagemigration_GroupVersionResource(in, out, s)
}

func autoConvert_storagemigration_GroupVersionResource_To_v1alpha1_GroupVersionResource(in *storagemigration.GroupVersionResource, out *storagemigrationv1alpha1.GroupVersionResource, s conversion.Scope) error {
	out.Group = in.Group
	out.Version = in.Version
	out.Resource = in.Resource
	return nil
}

// Convert_storagemigration_GroupVersionResource_To_v1alpha1_GroupVersionResource is an autogenerated conversion function.
func Convert_storagemigration_GroupVersionResource_To_v1alpha1_GroupVersionResource(in *storagemigration.GroupVersionResource, out *storagemigrationv1alpha1.GroupVersionResource, s conversion.Scope) error {
	return autoConvert_storagemigration_GroupVersionResource_To_v1alpha1_GroupVersionResource(in, out, s)
}

func autoConvert_v1alpha1_MigrationCondition_To_storagemigration_MigrationCondition(in *storagemigrationv1alpha1.MigrationCondition, out *storagemigration.MigrationCondition, s conversion.Scope) error {
	out.Type = storagemigration.MigrationConditionType(in.Type)
	out.Status = v1.ConditionStatus(in.Status)
	out.LastUpdateTime = in.LastUpdateTime
	out.Reason = in.Reason
	out.Message = in.Message
	return nil
}

// Convert_v1alpha1_MigrationCondition_To_storagemigration_MigrationCondition is an autogenerated conversion function.
func Convert_v1alpha1_MigrationCondition_To_storagemigration_MigrationCondition(in *storagemigrationv1alpha1.MigrationCondition, out *storagemigration.MigrationCondition, s conversion.Scope) error {
	return autoConvert_v1alpha1_MigrationCondition_To_storagemigration_MigrationCondition(in, out, s)
}

func autoConvert_storagemigration_MigrationCondition_To_v1alpha1_MigrationCondition(in *storagemigration.MigrationCondition, out *storagemigrationv1alpha1.MigrationCondition, s conversion.Scope) error {
	out.Type = storagemigrationv1alpha1.MigrationConditionType(in.Type)
	out.Status = v1.ConditionStatus(in.Status)
	out.LastUpdateTime = in.LastUpdateTime
	out.Reason = in.Reason
	out.Message = in.Message
	return nil
}

// Convert_storagemigration_MigrationCondition_To_v1alpha1_MigrationCondition is an autogenerated conversion function.
func Convert_storagemigration_MigrationCondition_To_v1alpha1_MigrationCondition(in *storagemigration.MigrationCondition, out *storagemigrationv1alpha1.MigrationCondition, s conversion.Scope) error {
	return autoConvert_storagemigration_MigrationCondition_To_v1alpha1_MigrationCondition(in, out, s)
}

func autoConvert_v1alpha1_StorageVersionMigration_To_storagemigration_StorageVersionMigration(in *storagemigrationv1alpha1.StorageVersionMigration, out *storagemigration.StorageVersionMigration, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	if err := Convert_v1alpha1_StorageVersionMigrationSpec_To_storagemigration_StorageVersionMigrationSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_v1alpha1_StorageVersionMigrationStatus_To_storagemigration_StorageVersionMigrationStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1alpha1_StorageVersionMigration_To_storagemigration_StorageVersionMigration is an autogenerated conversion function.
func Convert_v1alpha1_StorageVersionMigration_To_storagemigration_StorageVersionMigration(in *storagemigrationv1alpha1.StorageVersionMigration, out *storagemigration.StorageVersionMigration, s conversion.Scope) error {
	return autoConvert_v1alpha1_StorageVersionMigration_To_storagemigration_StorageVersionMigration(in, out, s)
}

func autoConvert_storagemigration_StorageVersionMigration_To_v1alpha1_StorageVersionMigration(in *storagemigration.StorageVersionMigration, out *storagemigrationv1alpha1.StorageVersionMigration, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	if err := Convert_storagemigration_StorageVersionMigrationSpec_To_v1alpha1_StorageVersionMigrationSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_storagemigration_StorageVersionMigrationStatus_To_v1alpha1_StorageVersionMigrationStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

// Convert_storagemigration_StorageVersionMigration_To_v1alpha1_StorageVersionMigration is an autogenerated conversion function.
func Convert_storagemigration_StorageVersionMigration_To_v1alpha1_StorageVersionMigration(in *storagemigration.StorageVersionMigration, out *storagemigrationv1alpha1.StorageVersionMigration, s conversion.Scope) error {
	return autoConvert_storagemigration_StorageVersionMigration_To_v1alpha1_StorageVersionMigration(in, out, s)
}

func autoConvert_v1alpha1_StorageVersionMigrationList_To_storagemigration_StorageVersionMigrationList(in *storagemigrationv1alpha1.StorageVersionMigrationList, out *storagemigration.StorageVersionMigrationList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	out.Items = *(*[]storagemigration.StorageVersionMigration)(unsafe.Pointer(&in.Items))
	return nil
}

// Convert_v1alpha1_StorageVersionMigrationList_To_storagemigration_StorageVersionMigrationList is an autogenerated conversion function.
func Convert_v1alpha1_StorageVersionMigrationList_To_storagemigration_StorageVersionMigrationList(in *storagemigrationv1alpha1.StorageVersionMigrationList, out *storagemigration.StorageVersionMigrationList, s conversion.Scope) error {
	return autoConvert_v1alpha1_StorageVersionMigrationList_To_storagemigration_StorageVersionMigrationList(in, out, s)
}

func autoConvert_storagemigration_StorageVersionMigrationList_To_v1alpha1_StorageVersionMigrationList(in *storagemigration.StorageVersionMigrationList, out *storagemigrationv1alpha1.StorageVersionMigrationList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	out.Items = *(*[]storagemigrationv1alpha1.StorageVersionMigration)(unsafe.Pointer(&in.Items))
	return nil
}

// Convert_storagemigration_StorageVersionMigrationList_To_v1alpha1_StorageVersionMigrationList is an autogenerated conversion function.
func Convert_storagemigration_StorageVersionMigrationList_To_v1alpha1_StorageVersionMigrationList(in *storagemigration.StorageVersionMigrationList, out *storagemigrationv1alpha1.StorageVersionMigrationList, s conversion.Scope) error {
	return autoConvert_storagemigration_StorageVersionMigrationList_To_v1alpha1_StorageVersionMigrationList(in, out, s)
}

func autoConvert_v1alpha1_StorageVersionMigrationSpec_To_storagemigration_StorageVersionMigrationSpec(in *storagemigrationv1alpha1.StorageVersionMigrationSpec, out *storagemigration.StorageVersionMigrationSpec, s conversion.Scope) error {
	if err := Convert_v1alpha1_GroupVersionResource_To_storagemigration_GroupVersionResource(&in.Resource, &out.Resource, s); err != nil {
		return err
	}
	out.ContinueToken = in.ContinueToken
	return nil
}

// Convert_v1alpha1_StorageVersionMigrationSpec_To_storagemigration_StorageVersionMigrationSpec is an autogenerated conversion function.
func Convert_v1alpha1_StorageVersionMigrationSpec_To_storagemigration_StorageVersionMigrationSpec(in *storagemigrationv1alpha1.StorageVersionMigrationSpec, out *storagemigration.StorageVersionMigrationSpec, s conversion.Scope) error {
	return autoConvert_v1alpha1_StorageVersionMigrationSpec_To_storagemigration_StorageVersionMigrationSpec(in, out, s)
}

func autoConvert_storagemigration_StorageVersionMigrationSpec_To_v1alpha1_StorageVersionMigrationSpec(in *storagemigration.StorageVersionMigrationSpec, out *storagemigrationv1alpha1.StorageVersionMigrationSpec, s conversion.Scope) error {
	if err := Convert_storagemigration_GroupVersionResource_To_v1alpha1_GroupVersionResource(&in.Resource, &out.Resource, s); err != nil {
		return err
	}
	out.ContinueToken = in.ContinueToken
	return nil
}

// Convert_storagemigration_StorageVersionMigrationSpec_To_v1alpha1_StorageVersionMigrationSpec is an autogenerated conversion function.
func Convert_storagemigration_StorageVersionMigrationSpec_To_v1alpha1_StorageVersionMigrationSpec(in *storagemigration.StorageVersionMigrationSpec, out *storagemigrationv1alpha1.StorageVersionMigrationSpec, s conversion.Scope) error {
	return autoConvert_storagemigration_StorageVersionMigrationSpec_To_v1alpha1_StorageVersionMigrationSpec(in, out, s)
}

func autoConvert_v1alpha1_StorageVersionMigrationStatus_To_storagemigration_StorageVersionMigrationStatus(in *storagemigrationv1alpha1.StorageVersionMigrationStatus, out *storagemigration.StorageVersionMigrationStatus, s conversion.Scope) error {
	out.Conditions = *(*[]storagemigration.MigrationCondition)(unsafe.Pointer(&in.Conditions))
	out.ResourceVersion = in.ResourceVersion
	return nil
}

// Convert_v1alpha1_StorageVersionMigrationStatus_To_storagemigration_StorageVersionMigrationStatus is an autogenerated conversion function.
func Convert_v1alpha1_StorageVersionMigrationStatus_To_storagemigration_StorageVersionMigrationStatus(in *storagemigrationv1alpha1.StorageVersionMigrationStatus, out *storagemigration.StorageVersionMigrationStatus, s conversion.Scope) error {
	return autoConvert_v1alpha1_StorageVersionMigrationStatus_To_storagemigration_StorageVersionMigrationStatus(in, out, s)
}

func autoConvert_storagemigration_StorageVersionMigrationStatus_To_v1alpha1_StorageVersionMigrationStatus(in *storagemigration.StorageVersionMigrationStatus, out *storagemigrationv1alpha1.StorageVersionMigrationStatus, s conversion.Scope) error {
	out.Conditions = *(*[]storagemigrationv1alpha1.MigrationCondition)(unsafe.Pointer(&in.Conditions))
	out.ResourceVersion = in.ResourceVersion
	return nil
}

// Convert_storagemigration_StorageVersionMigrationStatus_To_v1alpha1_StorageVersionMigrationStatus is an autogenerated conversion function.
func Convert_storagemigration_StorageVersionMigrationStatus_To_v1alpha1_StorageVersionMigrationStatus(in *storagemigration.StorageVersionMigrationStatus, out *storagemigrationv1alpha1.StorageVersionMigrationStatus, s conversion.Scope) error {
	return autoConvert_storagemigration_StorageVersionMigrationStatus_To_v1alpha1_StorageVersionMigrationStatus(in, out, s)
}
