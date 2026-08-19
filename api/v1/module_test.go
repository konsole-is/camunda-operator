package v1_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The api module exists so that a consumer gets the CRD types without the
// dependencies of the operator. This test keeps that promise: the build graph
// of the module holds apimachinery, k8s.io/api, and nothing from the operator
// or its framework.
func TestModuleDependencies(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	require.NoError(t, err)

	deps := strings.Split(strings.TrimSpace(string(out)), "\n")
	require.NotEmpty(t, deps)

	for _, dep := range deps {
		for _, forbidden := range []string{
			"sigs.k8s.io/controller-runtime",
			"github.com/sourcehawk/operator-component-framework",
			"github.com/konsole-is/camunda-operator/pkg",
			"github.com/konsole-is/camunda-operator/internal",
		} {
			assert.False(t, strings.HasPrefix(dep, forbidden), "api module must not depend on %s", dep)
		}
	}
}
