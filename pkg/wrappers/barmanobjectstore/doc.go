/*
Copyright 2026.

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

// Package barmanobjectstore holds the Go types of the ObjectStore custom
// resource of the Barman Cloud plugin for CloudNativePG,
// barmancloud.cnpg.io/v1, and the ocf primitive that reconciles it.
//
// The plugin does publish a Go module, but importing it would add the whole
// CloudNativePG operator, cert-manager, cnpg-i, and grpc to this operator's
// dependencies, so the types here are copied from the CRD instead. The schema
// they follow is vendored in internal/testenv/crds/barmancloud.
//
// Each type in types.go carries the object:generate marker of its own. The
// marker of the whole package would also reach the wrapper, whose mutation
// type names an ocf interface that controller-gen cannot copy.
// +groupName=barmancloud.cnpg.io
package barmanobjectstore
