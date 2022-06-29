/*
Copyright 2022 The Kubernetes Authors.

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

package v2alpha1

// +k8s:protobuf-gen=package

// EncryptedObject is the representation of data stored in etcd after envelope encryption.
type EncryptedObject struct {
	// EncryptedData is the encrypted data.
	EncryptedData []byte `json:"data" protobuf:"bytes,2,opt,name=encryptedData"`
	// KeyID is the KMS key ID used for encryption operations.
	KeyID string `protobuf:"bytes,3,opt,name=keyID" json:"keyID,omitempty"`
	// PluginName is the name of the KMS plugin used for encryption.
	PluginName string `protobuf:"bytes,4,opt,name=pluginName" json:"pluginName,omitempty"`
	// EncryptedDEK is the encrypted DEK.
	EncryptedDEK []byte `protobuf:"bytes,5,opt,name=encryptedDEK" json:"encryptedDEK,omitempty"`
	// Annotations is additional metadata that was provided by the KMS plugin.
	Annotations map[string][]byte `protobuf:"bytes,6,rep,name=annotations" json:"annotations,omitempty"`
}
