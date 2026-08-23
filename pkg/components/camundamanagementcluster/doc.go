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

// Package camundamanagementcluster renders the resources of one management
// plane: the Management Identity Deployment and Service, the copies of
// referenced Secrets from other namespaces, and later the identity provider,
// Console, and Web Modeler. It also derives the ManagementAuthConfig that
// Optimize reads.
//
// The package is pure: spec in, resources out, no API calls. The controller in
// internal/controller/camundamanagementcluster resolves the references into
// Input and applies what this package renders.
package camundamanagementcluster
