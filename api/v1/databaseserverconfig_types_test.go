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

package v1_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestProbedForCurrentSpec(t *testing.T) {
	t.Parallel()

	probed := func(mutate func(*v1.DatabaseServerConfig)) *v1.DatabaseServerConfig {
		cfg := &v1.DatabaseServerConfig{
			Spec: v1.DatabaseServerConfigSpec{
				Host: "my-db-rw",
				Port: 5432,
				AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
					Name: "my-db-superuser", UsernameKey: "username", PasswordKey: "password",
				},
			},
			Status: v1.DatabaseServerConfigStatus{
				SystemIdentifier:    "7412345678901234567",
				ProbedAt:            &metav1.Time{Time: time.Unix(0, 0)},
				ProbedEndpoint:      "my-db-rw:5432",
				ProbedSecretName:    "my-db-superuser",
				ProbedSecretKeys:    "username/password",
				ProbedSecretVersion: "1",
			},
		}
		if mutate != nil {
			mutate(cfg)
		}

		return cfg
	}

	tests := []struct {
		name string
		cfg  *v1.DatabaseServerConfig
		want bool
	}{
		{
			name: "the record describes the endpoint and the credentials of the spec",
			cfg:  probed(nil),
			want: true,
		},
		{
			name: "no probe has run",
			cfg:  probed(func(cfg *v1.DatabaseServerConfig) { cfg.Status = v1.DatabaseServerConfigStatus{} }),
			want: false,
		},
		{
			name: "the spec moved to another host",
			cfg:  probed(func(cfg *v1.DatabaseServerConfig) { cfg.Spec.Host = "other-db-rw" }),
			want: false,
		},
		{
			name: "the spec moved to another port",
			cfg:  probed(func(cfg *v1.DatabaseServerConfig) { cfg.Spec.Port = 6432 }),
			want: false,
		},
		{
			name: "the spec names another admin credentials Secret",
			cfg: probed(func(cfg *v1.DatabaseServerConfig) {
				cfg.Spec.AdminCredentialsSecretRef.Name = "other-superuser"
			}),
			want: false,
		},
		{
			name: "the spec names another user in the same Secret",
			cfg: probed(func(cfg *v1.DatabaseServerConfig) {
				cfg.Spec.AdminCredentialsSecretRef.UsernameKey = "admin"
			}),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.cfg.ProbedForCurrentSpec())
		})
	}
}
