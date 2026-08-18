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

package credentials

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// passwordPattern is the shape that every generated password must have.
var passwordPattern = regexp.MustCompile(`^[A-Za-z0-9]{32}$`)

func TestNewPasswordIs32AlphanumericChars(t *testing.T) {
	t.Parallel()

	password, err := NewPassword()
	require.NoError(t, err)

	assert.Regexp(t, passwordPattern, password)
}

func TestNewPasswordIsUniquePerCall(t *testing.T) {
	t.Parallel()

	first, err := NewPassword()
	require.NoError(t, err)
	second, err := NewPassword()
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
}

func TestLookupOrNew(t *testing.T) {
	t.Parallel()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "ns", UID: types.UID("uid-1")},
		Data:       map[string][]byte{"password": []byte("s3cret"), "empty": {}},
	}
	key := client.ObjectKey{Name: "creds", Namespace: "ns"}

	tests := []struct {
		name   string
		reader client.Reader
		field  string
		// wantExisting reports whether the stored value must come back. When it
		// is false, a new password is generated instead.
		wantExisting bool
	}{
		{
			name:         "existing field is reused",
			reader:       fake.NewClientBuilder().WithObjects(secret).Build(),
			field:        "password",
			wantExisting: true,
		},
		{
			name:   "missing Secret yields a new password",
			reader: fake.NewClientBuilder().Build(),
			field:  "password",
		},
		{
			name:   "missing field yields a new password",
			reader: fake.NewClientBuilder().WithObjects(secret).Build(),
			field:  "username",
		},
		{
			name:   "empty field yields a new password",
			reader: fake.NewClientBuilder().WithObjects(secret).Build(),
			field:  "empty",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			password, err := LookupOrNew(context.Background(), tt.reader, key, tt.field)
			require.NoError(t, err)
			if tt.wantExisting {
				assert.Equal(t, "s3cret", password.Value)
				assert.Equal(
					t, types.UID("uid-1"), password.SourceUID,
					"a reused password must name the Secret it came from",
				)
				return
			}
			assert.Regexp(t, passwordPattern, password.Value)
			assert.Empty(
				t, password.SourceUID,
				"a new password has no source object, so the apply must be free to create the Secret",
			)
		})
	}
}

func TestPasswordPreconditionAnnotations(t *testing.T) {
	t.Parallel()

	reused := Password{Value: "s3cret", SourceUID: types.UID("uid-1")}
	assert.Equal(
		t,
		map[string]string{PreconditionAnnotation: "uid-1"},
		reused.PreconditionAnnotations(),
	)

	assert.Nil(t, Password{Value: "s3cret"}.PreconditionAnnotations())
}

func TestLookupOrNewReturnsTransientErrors(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	reader := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
			return boom
		},
	}).Build()

	_, err := LookupOrNew(context.Background(), reader, client.ObjectKey{Name: "creds", Namespace: "ns"}, "password")
	require.ErrorIs(t, err, boom)
}
