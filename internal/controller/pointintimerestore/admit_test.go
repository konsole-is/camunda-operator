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

package pointintimerestore

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// admitNow is the clock of every case below.
var admitNow = time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)

// ago returns the point d before admitNow.
func ago(d time.Duration) time.Time {
	return admitNow.Add(-d)
}

// contractFrom returns a contract with a retention of seven days and the
// given recoverableFrom.
func contractFrom(from *metav1.Time) *v1.DatabaseServerConfig {
	return &v1.DatabaseServerConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "dbsc", Namespace: "my-cluster-ns"},
		Spec: v1.DatabaseServerConfigSpec{
			PITR: &v1.PITRCapability{
				Enabled:             true,
				RetentionPeriodDays: new(int32(7)),
				RecoverableFrom:     from,
			},
		},
	}
}

func TestPitrAvailableChecksRecoverableFrom(t *testing.T) {
	t.Parallel()

	from := new(metav1.NewTime(ago(3 * day)))

	tests := []struct {
		name     string
		want     time.Time
		accepted bool
	}{
		{name: "a point before recoverableFrom", want: ago(3*day + time.Second)},
		{name: "recoverableFrom itself", want: ago(3 * day), accepted: true},
		{name: "a point after recoverableFrom", want: ago(time.Hour), accepted: true},
		{name: "the present", want: admitNow, accepted: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			failure := pitrAvailable(contractFrom(from), tt.want, admitNow)
			if tt.accepted {
				assert.Nil(t, failure)
				return
			}

			require.NotNil(t, failure)
			assert.Equal(t, v1.ReasonPitrUnavailable, failure.Reason)
			assert.Contains(t, failure.Message, tt.want.UTC().Format(time.RFC3339))
			assert.Contains(t, failure.Message, "recoverableFrom "+from.UTC().Format(time.RFC3339))
			assert.Contains(t, failure.Message, "my-cluster-ns/dbsc")
		})
	}
}

// A contract without recoverableFrom keeps the rules of the retention period
// and the present. A managed database publishes that shape.
func TestPitrAvailableWithoutRecoverableFrom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		want     time.Time
		contains string
	}{
		{name: "a point inside the retention period", want: ago(6 * day)},
		{name: "the present", want: admitNow},
		{name: "a point in the future", want: admitNow.Add(time.Second), contains: "future"},
		{name: "a point older than the retention period", want: ago(8 * day), contains: "7 days"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			failure := pitrAvailable(contractFrom(nil), tt.want, admitNow)
			if tt.contains == "" {
				assert.Nil(t, failure)
				return
			}

			require.NotNil(t, failure)
			assert.Equal(t, v1.ReasonPitrUnavailable, failure.Reason)
			assert.Contains(t, failure.Message, tt.contains)
		})
	}
}

// recoverableFrom narrows the retention period, never widens it: a point
// after recoverableFrom and older than the retention period is still refused,
// and so is a point in the future.
func TestPitrAvailableKeepsTheRetentionPeriodAndThePresent(t *testing.T) {
	t.Parallel()

	contract := contractFrom(new(metav1.NewTime(ago(10 * day))))

	old := pitrAvailable(contract, ago(8*day), admitNow)
	require.NotNil(t, old)
	assert.Contains(t, old.Message, "7 days")

	future := pitrAvailable(contract, admitNow.Add(time.Second), admitNow)
	require.NotNil(t, future)
	assert.Contains(t, future.Message, "future")
}
