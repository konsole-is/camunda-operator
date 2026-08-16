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

package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCamundaPlatformConfigSpecMethod(t *testing.T) {
	tests := []struct {
		name string
		spec CamundaPlatformConfigSpec
		want AuthenticationMethod
	}{
		{name: "auth unset is basic", spec: CamundaPlatformConfigSpec{}, want: AuthenticationMethodBasic},
		{
			name: "auth set without method is basic",
			spec: CamundaPlatformConfigSpec{Auth: &PlatformAuthSpec{}},
			want: AuthenticationMethodBasic,
		},
		{
			name: "explicit basic",
			spec: CamundaPlatformConfigSpec{Auth: &PlatformAuthSpec{Method: AuthenticationMethodBasic}},
			want: AuthenticationMethodBasic,
		},
		{
			name: "explicit oidc",
			spec: CamundaPlatformConfigSpec{Auth: &PlatformAuthSpec{Method: AuthenticationMethodOIDC}},
			want: AuthenticationMethodOIDC,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.spec.Method())
		})
	}
}
