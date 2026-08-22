package camundacluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionDowngrade(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		effective, running string
		want               bool
	}{
		"patch below":           {"8.9.8", "8.9.9", true},
		"minor below":           {"8.9.9", "8.10.0", true},
		"major below":           {"8.10.0", "9.0.0", true},
		"same version":          {"8.9.9", "8.9.9", false},
		"minor above":           {"8.10.0", "8.9.9", false},
		"no running version":    {"8.9.8", "", false},
		"running tag not x.y.z": {"8.9.8", "latest", false},
		"effective not x.y.z":   {"8.9", "8.9.9", false},
		"numeric, not lexical":  {"8.9.10", "8.9.9", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, VersionDowngrade(tc.effective, tc.running))
		})
	}
}

func TestImageTag(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ image, want string }{
		"plain":              {"camunda/camunda:8.9.9", "8.9.9"},
		"registry with port": {"registry.example.com:5000/camunda/camunda:8.9.9", "8.9.9"},
		"digest after tag":   {"camunda/camunda:8.9.9@sha256:abc", "8.9.9"},
		"no tag":             {"camunda/camunda", ""},
		"port but no tag":    {"registry.example.com:5000/camunda/camunda", ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, ImageTag(tc.image))
		})
	}
}
