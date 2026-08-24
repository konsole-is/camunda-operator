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

package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// collidingDatabase returns a Database fixture with the given name and
// creation time. All fixtures share serverRef and databaseName.
func collidingDatabase(name string, created time.Time) v1.Database {
	return v1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(created),
		},
		Spec: v1.DatabaseSpec{
			ServerRef:    "shared-server",
			DatabaseName: "camunda",
		},
	}
}

// TestCollisionKeyIsTheServerIdentity pins that the claim belongs to the
// PostgreSQL instance, not to the contract that describes it. Two contracts
// of two namespaces that reach one instance produce one key.
func TestCollisionKeyIsTheServerIdentity(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "7000000000000000001/camunda", CollisionKey("7000000000000000001", "camunda"))
	assert.Equal(
		t,
		CollisionKey("7000000000000000001", "camunda"),
		CollisionKey("7000000000000000001", "camunda"),
	)
	assert.NotEqual(
		t,
		CollisionKey("7000000000000000001", "camunda"),
		CollisionKey("7000000000000000002", "camunda"),
	)
	assert.NotEqual(
		t,
		CollisionKey("7000000000000000001", "camunda"),
		CollisionKey("7000000000000000001", "identity"),
	)
}

func TestCollisionWinner(t *testing.T) {
	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		items []v1.Database
		want  string
	}{
		{
			name:  "single claimant wins",
			items: []v1.Database{collidingDatabase("only", base)},
			want:  "only",
		},
		{
			name: "oldest creation wins over name order",
			items: []v1.Database{
				collidingDatabase("aaa-later", base.Add(time.Hour)),
				collidingDatabase("zzz-first", base),
			},
			want: "zzz-first",
		},
		{
			name: "tie on creation time falls back to name order",
			items: []v1.Database{
				collidingDatabase("bbb", base),
				collidingDatabase("aaa", base),
			},
			want: "aaa",
		},
		{
			name: "the older claimant of another namespace still wins",
			items: []v1.Database{
				func() v1.Database {
					db := collidingDatabase("aaa", base.Add(time.Hour))
					db.Namespace = "alpha"
					return db
				}(),
				func() v1.Database {
					db := collidingDatabase("zzz", base)
					db.Namespace = "omega"
					return db
				}(),
			},
			want: "zzz",
		},
		{
			name: "three claimants",
			items: []v1.Database{
				collidingDatabase("ccc", base.Add(time.Minute)),
				collidingDatabase("bbb", base),
				collidingDatabase("aaa", base.Add(time.Hour)),
			},
			want: "bbb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			winner := CollisionWinner(tt.items)
			require.NotNil(t, winner)
			assert.Equal(t, tt.want, winner.Name)
		})
	}

	assert.Nil(t, CollisionWinner(nil), "no claimants means no winner")
}
