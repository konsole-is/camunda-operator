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

package conformance

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A controller that builds a ReconcileContext without Metrics records no
// condition gauge and no apply counter, and nothing at runtime reports that.
// A controller whose recorder name differs from its controller-runtime name
// records series the dashboards cannot join. A controller that never calls
// Forget leaves the series of a deleted owner on /metrics. All three are
// invisible in a cluster, so this test reads the source of every controller
// package instead.
func TestEveryControllerRecordsMetricsUnderItsControllerName(t *testing.T) {
	t.Parallel()

	controllerDir := ".."
	packages, err := controllerPackages(controllerDir)
	require.NoError(t, err)
	require.NotEmpty(t, packages, "no controller package found under %s", controllerDir)

	for _, pkg := range packages {
		t.Run(pkg, func(t *testing.T) {
			t.Parallel()

			w := parseControllerPackage(t, filepath.Join(controllerDir, pkg))
			assert.NotEmpty(t, w.contexts, "no ReconcileContext literal found")
			for _, pos := range w.contextsWithoutMetrics {
				assert.Fail(t, "ReconcileContext literal does not set Metrics", pos)
			}

			assert.NotEmpty(t, w.names["Recorder"], "no observability.Recorder call found")
			assert.True(t, w.forgets, "no observability.Forget call found: a deleted owner keeps its series")
			assert.NotEmpty(t, w.names["Named"], "no Named call found")
			assert.Len(
				t,
				distinct(w.names),
				1,
				"Named, observability.Recorder, and GetEventRecorder where present must take one name: %v",
				w.names,
			)
		})
	}
}

// controllerPackages returns, relative to root, every directory under root
// that holds a Go file other than a test. Such a directory is a controller
// package. A directory without one, such as a group directory or a package
// of schema specs, is walked into and not returned.
func controllerPackages(root string) ([]string, error) {
	var packages []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return err
		}

		sources, err := filepath.Glob(filepath.Join(path, "*.go"))
		if err != nil {
			return err
		}

		for _, source := range sources {
			if strings.HasSuffix(source, "_test.go") {
				continue
			}

			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}

			packages = append(packages, rel)
			break
		}
		return nil
	})

	return packages, err
}

// wiring is what one controller package says about its metrics.
type wiring struct {
	contexts               []string
	contextsWithoutMetrics []string
	forgets                bool
	// names maps a call name to the source text of its first argument, one
	// entry per call site.
	names map[string][]string
}

func parseControllerPackage(t *testing.T, dir string) wiring {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	require.NoError(t, err)

	fset := token.NewFileSet()
	w := wiring{names: map[string][]string{}}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, err)

		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CompositeLit:
				if isSelector(node.Type, "component", "ReconcileContext") {
					pos := fset.Position(node.Pos()).String()
					w.contexts = append(w.contexts, pos)
					if !setsKey(node, "Metrics") {
						w.contextsWithoutMetrics = append(w.contextsWithoutMetrics, pos)
					}
				}
			case *ast.CallExpr:
				if isSelector(node.Fun, "observability", "Forget") {
					w.forgets = true
				}
				name, ok := wiringCall(node)
				if ok && len(node.Args) > 0 {
					w.names[name] = append(w.names[name], sourceOf(t, path, fset, node.Args[0]))
				}
			}
			return true
		})
	}
	return w
}

// wiringCall reports the calls whose first argument is a controller name.
func wiringCall(call *ast.CallExpr) (string, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}

	switch sel.Sel.Name {
	case "Named", "GetEventRecorder":
		return sel.Sel.Name, true
	case "Recorder":
		return sel.Sel.Name, isSelector(sel, "observability", "Recorder")
	}
	return "", false
}

func isSelector(expr ast.Expr, pkg, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}

	ident, ok := sel.X.(*ast.Ident)
	return ok && ident.Name == pkg
}

func setsKey(lit *ast.CompositeLit, key string) bool {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if ident, ok := kv.Key.(*ast.Ident); ok && ident.Name == key {
			return true
		}
	}
	return false
}

func sourceOf(t *testing.T, path string, fset *token.FileSet, expr ast.Expr) string {
	t.Helper()

	src, err := os.ReadFile(filepath.Clean(path))
	require.NoError(t, err)

	return string(src[fset.Position(expr.Pos()).Offset:fset.Position(expr.End()).Offset])
}

func distinct(names map[string][]string) []string {
	seen := map[string]struct{}{}
	for _, list := range names {
		for _, name := range list {
			seen[name] = struct{}{}
		}
	}

	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	return out
}

// A group directory such as fleet holds no source of its own, and a package of
// schema specs holds tests only. Neither runs a controller, and a controller
// package below either of them must still be checked.
func TestControllerPackagesWalksIntoDirectoriesWithoutSource(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, file := range []string{
		"database/controller.go",
		"database/withoutcnpg/suite_test.go",
		"fleet/environment/controller.go",
		"fleet/schema/suite_test.go",
		"conformance/suite_test.go",
	} {
		path := filepath.Join(root, file)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte("package x\n"), 0o600))
	}

	packages, err := controllerPackages(root)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"database", filepath.Join("fleet", "environment")}, packages)
}
