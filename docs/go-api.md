# Use the API types from Go

The CRD types of this operator are a Go module of their own:
`github.com/konsole-is/camunda-operator/api`. A program that creates or reads a
`CamundaCluster`, a contract CR, or a backup CR imports this module. It does not
import the operator.

The module depends on `k8s.io/api` and `k8s.io/apimachinery` only. It does not
depend on controller-runtime, on the operator component framework, or on the
operator packages under `pkg/`. Your program gets at least the versions of
`k8s.io/api` and `k8s.io/apimachinery` that the `go.mod` of the api module
requires.

## Get the module

```bash
go get github.com/konsole-is/camunda-operator/api@v<version>
```

Replace `<version>` with the version of an operator release, for example
`0.1.0` for the release `v0.1.0`. The api module and the operator share one
version number: the release `v0.1.0` of the operator publishes the Go tag
`api/v0.1.0`.

## Import the types

```go
import (
    "k8s.io/apimachinery/pkg/runtime"

    camundav1 "github.com/konsole-is/camunda-operator/api/v1"
)

func newScheme() (*runtime.Scheme, error) {
    scheme := runtime.NewScheme()
    if err := camundav1.AddToScheme(scheme); err != nil {
        return nil, err
    }

    return scheme, nil
}
```

`camundav1.AddToScheme` registers every kind of the `core.camunda.io/v1` group.
After that, a controller-runtime client or a typed client can read and write
the resources.

## The operator module

The root module `github.com/konsole-is/camunda-operator` holds the controllers
and the shared packages under `pkg/`. The release tag `v<version>` is also
its Go tag, and the root module requires the api module at the same
version. A program that needs a package from `pkg/` imports the root
module at a release tag. That program gets every dependency of the operator.

Commits between two releases require the api version of the last release. A
program that pins the root module at a commit on `main` must also require the
api module at the same commit.
