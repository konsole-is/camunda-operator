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

package secretref

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// secret builds a Secret at "ns/name" holding the given keys.
func secret(keys ...string) *corev1.Secret {
	data := make(map[string][]byte, len(keys))
	for _, key := range keys {
		data[key] = []byte("value")
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "name", Namespace: "ns"},
		Data:       data,
	}
}

func TestCheckKeys(t *testing.T) {
	tests := []struct {
		name    string
		objects []*corev1.Secret
		ref     types.NamespacedName
		keys    []string
		want    string
	}{
		{
			name: "secret absent",
			ref:  types.NamespacedName{Namespace: "ns", Name: "name"},
			keys: []string{"username", "password"},
			want: "Secret ns/name not found",
		},
		{
			name:    "secret present but key absent",
			objects: []*corev1.Secret{secret("username")},
			ref:     types.NamespacedName{Namespace: "ns", Name: "name"},
			keys:    []string{"username", "password"},
			want:    `Secret ns/name is missing key "password"`,
		},
		{
			name:    "all keys present",
			objects: []*corev1.Secret{secret("username", "password")},
			ref:     types.NamespacedName{Namespace: "ns", Name: "name"},
			keys:    []string{"username", "password"},
			want:    "",
		},
		{
			name:    "message names the first missing key",
			objects: []*corev1.Secret{secret("c")},
			ref:     types.NamespacedName{Namespace: "ns", Name: "name"},
			keys:    []string{"a", "b", "c"},
			want:    `Secret ns/name is missing key "a"`,
		},
		{
			name:    "no keys requested",
			objects: []*corev1.Secret{secret()},
			ref:     types.NamespacedName{Namespace: "ns", Name: "name"},
			keys:    nil,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder()
			for _, obj := range tt.objects {
				builder = builder.WithObjects(obj)
			}
			reader := builder.Build()

			msg, err := CheckKeys(context.Background(), reader, tt.ref, tt.keys...)

			require.NoError(t, err)
			assert.Equal(t, tt.want, msg)
		})
	}
}

// Get returns the Secret when every key is present, and the CheckKeys message
// otherwise, so a caller that needs the data reads the Secret once.
func TestGet(t *testing.T) {
	ref := types.NamespacedName{Namespace: "ns", Name: "name"}

	t.Run("secret absent", func(t *testing.T) {
		got, msg, err := Get(context.Background(), fake.NewClientBuilder().Build(), ref, "username")
		require.NoError(t, err)
		assert.Nil(t, got)
		assert.Equal(t, "Secret ns/name not found", msg)
	})

	t.Run("key absent", func(t *testing.T) {
		reader := fake.NewClientBuilder().WithObjects(secret("username")).Build()
		got, msg, err := Get(context.Background(), reader, ref, "username", "password")
		require.NoError(t, err)
		assert.Nil(t, got)
		assert.Equal(t, `Secret ns/name is missing key "password"`, msg)
	})

	t.Run("all keys present", func(t *testing.T) {
		reader := fake.NewClientBuilder().WithObjects(secret("username", "password")).Build()
		got, msg, err := Get(context.Background(), reader, ref, "username", "password")
		require.NoError(t, err)
		assert.Empty(t, msg)
		require.NotNil(t, got)
		assert.Equal(t, []byte("value"), got.Data["password"])
		assert.NotEmpty(t, got.ResourceVersion)
	})
}

func TestDataDigest(t *testing.T) {
	base := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "name", Namespace: "ns"},
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("secret"),
			"other":    []byte("unread"),
		},
	}
	digest := DataDigest(base, "username", "password")

	t.Run("key order does not matter", func(t *testing.T) {
		assert.Equal(t, digest, DataDigest(base, "password", "username"))
	})

	t.Run("metadata change keeps the digest", func(t *testing.T) {
		changed := base.DeepCopy()
		changed.Labels = map[string]string{"team": "a"}
		changed.Annotations = map[string]string{"reconciled-at": "now"}
		changed.ResourceVersion = "42"
		assert.Equal(t, digest, DataDigest(changed, "username", "password"))
	})

	t.Run("referenced value change moves the digest", func(t *testing.T) {
		changed := base.DeepCopy()
		changed.Data["password"] = []byte("rotated")
		assert.NotEqual(t, digest, DataDigest(changed, "username", "password"))
	})

	t.Run("unreferenced key change keeps the digest", func(t *testing.T) {
		changed := base.DeepCopy()
		changed.Data["other"] = []byte("changed")
		changed.Data["added"] = []byte("new")
		assert.Equal(t, digest, DataDigest(changed, "username", "password"))
	})

	t.Run("value moved to another key moves the digest", func(t *testing.T) {
		swapped := base.DeepCopy()
		swapped.Data["username"], swapped.Data["password"] = base.Data["password"], base.Data["username"]
		assert.NotEqual(t, digest, DataDigest(swapped, "username", "password"))
	})

	t.Run("no keys covers the whole data", func(t *testing.T) {
		whole := DataDigest(base)
		changed := base.DeepCopy()
		changed.Data["other"] = []byte("changed")
		assert.NotEqual(t, whole, DataDigest(changed))
	})

	t.Run("digest does not carry the value", func(t *testing.T) {
		assert.Len(t, digest, 16)
		assert.NotContains(t, digest, "secret")
		assert.NotContains(t, digest, "admin")
	})
}
