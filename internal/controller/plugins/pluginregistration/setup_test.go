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

package pluginregistration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupWithManagerRefusesAnOperatorVersionThatIsNotSemver(t *testing.T) {
	for _, v := range []string{"", "0.3", "v0.3.0", "dev"} {
		err := (&Reconciler{OperatorVersion: v}).SetupWithManager(nil)

		assert.Error(t, err, "operator version %q", v)
	}
}
