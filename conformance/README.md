# Plugin conformance suite

The Go module `github.com/konsole-is/camunda-operator/conformance` checks that a plugin controller keeps the contract of the group `plugins.camunda.io`. A plugin repository runs it from its own tests, against a cluster with the operator and the plugin installed. The section [Run the conformance suite](https://github.com/konsole-is/camunda-operator/blob/main/docs/guides/writing-a-plugin.md#run-the-conformance-suite) of the plugin guide lists each check and tells you what the cluster needs.

## Get the module

```bash
go get github.com/konsole-is/camunda-operator/conformance@v<version>
```

Replace `<version>` with the version of the operator release that your plugin builds against, for example `0.3.0`. The module requires the api module of the same release. It does not pull in the operator module or its dependencies.

## Run the checks from a test

`conformance.Run` runs each check as a subtest of your test, named by the check ID:

```go
package plugin_test

import (
	"testing"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, conformance.Config{
		Registration: "s3-backup-bucket",
		Namespace:    "conformance-s3-backup-bucket",
		Cases: []conformance.Case{
			{
				Name: "default",
				Target: pluginsv1alpha1.TargetRef{
					Kind: "CamundaCluster",
					Name: "orders",
				},
			},
		},
		Skip: []conformance.Skip{
			{ID: conformance.ReqLogs, Reason: "the controller writes plain text logs"},
		},
	})
}
```

A `Case` also takes the `Inputs` and the `Parameters` of the request, and a `DeletionPolicy`. A failed check fails the test, and the later checks still run. A run can skip only a check that the registry marks as skippable, and each skip needs a reason. If the config is not valid, or the suite cannot connect to the cluster, `Run` fails the test and runs no check.

`RESTConfig` connects the suite to the cluster. When it is nil, the suite uses the first of the `--kubeconfig` flag, the `KUBECONFIG` file, the in-cluster config, and `$HOME/.kube/config`.

## Run the checks from a program

`conformance.Execute` runs the same checks outside a test:

```go
results, err := conformance.Execute(ctx, cfg)
```

It returns one `Result` for each check, with the `ID`, the `Outcome` (`Passed`, `Failed`, or `Skipped`), the `Message`, and the `Duration`. A failed check is a result, not an error. `Execute` returns an error and runs no check when the config is not valid or when it cannot connect to the cluster. When the cleanup at the end of the run fails, it returns the results and an error.

## Timeouts

`Config.Timeouts` sets the longest waits of a run. A zero field takes its default:

| Field | Wait | Default |
| --- | --- | --- |
| `OutputsReady` | For `OutputsReady=True` on a request. | 5 minutes |
| `Ready` | For `Ready=True` on a request. | 10 minutes |
| `Deletion` | For each deletion: a request and its objects, a terminating namespace, and the cleanup. | 5 minutes |

## Other languages

A plugin in another language runs the binary `plugin-conformance` with a config file. See [Run the conformance suite](https://github.com/konsole-is/camunda-operator/blob/main/docs/guides/writing-a-plugin.md#run-the-conformance-suite).
