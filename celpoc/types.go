/*
Copyright 2021 The Kubernetes Authors.

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

package celpoc

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"

	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

var (
	kubeExt = runtime.RawExtension{}
)

type KubeObject struct {
	t         ref.Type
	adapter   ref.TypeAdapter
	provider  ref.TypeProvider
	raw       interface{}
	reflected reflect.Value
}

func (o *KubeObject) ConvertToNative(typeDesc reflect.Type) (interface{}, error) {
	return o.raw, nil
}

func (o *KubeObject) ConvertToType(typeValue ref.Type) ref.Val {
	if o.Type().TypeName() == typeValue.TypeName() {
		return o
	}
	return types.NewErr("conversion from %s to %s failed", o.t.TypeName(), typeValue.TypeName())
}

func (o *KubeObject) Equal(other ref.Val) ref.Val {
	otherObj, ok := other.(*KubeObject)
	if !ok {
		return types.MaybeNoSuchOverloadErr(other)
	}
	if o.Type().TypeName() != otherObj.Type().TypeName() {
		return types.NoSuchOverloadErr()
	}
	if reflect.DeepEqual(o.raw, otherObj.raw) {
		return types.True
	}
	return types.False
}

func (o *KubeObject) Get(index ref.Val) ref.Val {
	fieldName, ok := index.(types.String)
	if !ok {
		return types.MaybeNoSuchOverloadErr(index)
	}
	fieldStr := string(fieldName)
	ft, found := o.provider.FindFieldType(o.Type().TypeName(), fieldStr)
	if found {
		fv, err := ft.GetFrom(o.raw)
		if err != nil {
			return types.NewErr(err.Error())
		}
		return o.adapter.NativeToValue(fv)
	}
	objType := o.reflected.Type()
	fv := o.reflected.FieldByNameFunc(func(f string) bool {
		field, found := objType.FieldByName(f)
		if !found {
			return false
		}
		if jsonField, found := field.Tag.Lookup("json"); found {
			if strings.Split(jsonField, ",")[0] == fieldStr {
				return true
			}
		}
		return false
	})
	if fv.IsValid() {
		return o.adapter.NativeToValue(fv.Interface())
	}
	return o.adapter.NativeToValue(reflect.Zero(fv.Type()).Interface())
}

func (o *KubeObject) Type() ref.Type {
	return o.t
}

func (o *KubeObject) Value() interface{} {
	return o.raw
}

func NewKubeTypeProvider() *KubeTypeProvider {
	kubeTypes := []interface{}{
		new(v1.AdmissionRequest),
		new(v1.AdmissionResponse),
		new(corev1.Pod),
	}
	typeMap := make(map[string]*kubeType, len(kubeTypes))
	typeFields := map[string]map[string]*kubeFieldType{}
	// TODO: change the container to just use the standard package path with slashes
	// Possibly register both formats.
	for i := 0; i < len(kubeTypes); i++ {
		inst := kubeTypes[i]
		t := reflect.TypeOf(inst)
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		name := fmt.Sprintf("%s/%s", t.PkgPath(), t.Name())
		goName := strings.ReplaceAll(name, "/", ".")
		typeInst := &kubeType{
			exprType: decls.NewTypeType(decls.NewObjectType(name)),
			goType:   t,
		}
		typeMap[name] = typeInst
		typeMap[goName] = typeInst
		typeFieldMap := map[string]*kubeFieldType{}
		typeFields[name] = typeFieldMap
		typeFields[goName] = typeFieldMap
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			ft := f.Type
			if ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			jsonTag, ok := f.Tag.Lookup("json")
			if !ok {
				continue
			}
			fname := strings.Split(jsonTag, ",")[0]
			fieldType := kubeTypeToCelType(ft)
			typeFieldMap[fname] = &kubeFieldType{
				fieldType: &ref.FieldType{
					Type: fieldType,
					IsSet: func(target interface{}) bool {
						if kObj, ok := target.(*KubeObject); ok {
							target = kObj.Value()
						}
						fv := reflect.Indirect(reflect.ValueOf(target)).FieldByName(f.Name)
						return fv.IsValid()
					},
					GetFrom: func(target interface{}) (interface{}, error) {
						if kObj, ok := target.(*KubeObject); ok {
							target = kObj.Value()
						}
						fv := reflect.Indirect(reflect.ValueOf(target)).FieldByName(f.Name)
						if fv.IsValid() {
							return fv.Interface(), nil
						}
						return reflect.Zero(ft).Interface(), nil
					},
				},
				goName: f.Name,
			}
			if fieldType.GetMessageType() == "" {
				continue
			}
			if _, found := typeMap[fieldType.GetMessageType()]; found {
				continue
			}
			kubeTypes = append(kubeTypes, reflect.New(ft).Interface())
		}
	}
	tp := types.NewEmptyRegistry()
	return &KubeTypeProvider{
		TypeProvider: tp,
		typeMap:      typeMap,
		typeFields:   typeFields,
	}
}

type KubeTypeProvider struct {
	ref.TypeProvider
	typeMap    map[string]*kubeType
	typeFields map[string]map[string]*kubeFieldType
}

func (tp *KubeTypeProvider) FindType(typeName string) (*exprpb.Type, bool) {
	if t, found := tp.typeMap[typeName]; found {
		return t.exprType, found
	}
	return tp.TypeProvider.FindType(typeName)
}

func (tp *KubeTypeProvider) FindFieldType(typeName, fieldName string) (*ref.FieldType, bool) {
	if fields, found := tp.typeFields[typeName]; found {
		f, found := fields[fieldName]
		return f.fieldType, found
	}
	return tp.TypeProvider.FindFieldType(typeName, fieldName)
}

func (tp *KubeTypeProvider) NativeToValue(value interface{}) ref.Val {
	switch v := value.(type) {
	case ref.Val:
		return v
	case *unstructured.Unstructured:
		return tp.NativeToValue(v.Object)
	case runtime.RawExtension:
		return tp.NativeToValue(v.Object)
	}
	refVal := reflect.Indirect(reflect.ValueOf(value))
	switch refVal.Kind() {
	case reflect.Bool:
		return types.Bool(refVal.Convert(reflect.TypeOf(true)).Interface().(bool))
	case reflect.String:
		return types.String(refVal.Convert(reflect.TypeOf("")).Interface().(string))
	case reflect.Struct:
		typeName := fmt.Sprintf("%s.%s",
			strings.ReplaceAll(refVal.Type().PkgPath(), "/", "."),
			refVal.Type().Name())
		return &KubeObject{
			t:         types.NewObjectTypeValue(typeName),
			adapter:   tp,
			provider:  tp,
			raw:       value,
			reflected: refVal,
		}
	}
	return types.DefaultTypeAdapter.NativeToValue(value)
}

func (tp *KubeTypeProvider) NewValue(typeName string, fields map[string]ref.Val) ref.Val {
	typeInst := tp.typeMap[typeName]
	fieldTypes := tp.typeFields[typeName]
	refType := typeInst.goType
	refPtr := reflect.New(refType)
	refInst := refPtr.Elem()
	for field, val := range fields {
		ft := fieldTypes[field]
		refField := refInst.FieldByName(ft.goName)
		convVal, err := val.ConvertToNative(refField.Type())
		if err != nil {
			return types.NewErr(err.Error())
		}
		refField.Set(reflect.ValueOf(convVal))
	}
	return tp.NativeToValue(refPtr.Interface())
}

func kubeTypeToCelType(refType reflect.Type) *exprpb.Type {
	if refType.Kind() == reflect.Ptr {
		refType = refType.Elem()
	}
	switch refType.Kind() {
	case reflect.Bool:
		return decls.Bool
	case reflect.Float32, reflect.Float64:
		return decls.Double
	case reflect.Int, reflect.Int32, reflect.Int64:
		return decls.Int
	case reflect.String:
		return decls.String
	case reflect.Uint, reflect.Uint32, reflect.Uint64:
		return decls.Uint
	case reflect.Array, reflect.Slice:
		return decls.NewListType(kubeTypeToCelType(refType.Elem()))
	case reflect.Map:
		return decls.NewMapType(kubeTypeToCelType(refType.Key()), kubeTypeToCelType(refType.Elem()))
	case reflect.Struct:
		// Special case the runtime.RawExtension as a type Dyn
		if reflect.TypeOf(kubeExt) == refType {
			return decls.Dyn
		}
		return decls.NewObjectType(refType.Name())
	}
	return decls.Dyn
}

type kubeType struct {
	exprType *exprpb.Type
	goType   reflect.Type
}

type kubeFieldType struct {
	fieldType *ref.FieldType
	goName    string
}
