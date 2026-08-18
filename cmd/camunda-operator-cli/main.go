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

// camunda-operator-cli is the command-line companion of the operator. It is
// built and shipped as its own image, separate from the manager: the
// operator's Jobs run its subcommands as containers, and the manager learns
// which image that is through --camunda-operator-cli-image.
package main

import (
	"os"

	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/konsole-is/camunda-operator/internal/cli/deleteobject"
	"github.com/konsole-is/camunda-operator/internal/cli/upload"
)

func main() {
	// The signal handler cancels the context of every subcommand, so a
	// terminated pod abandons its work instead of committing a partial
	// result.
	if err := newRootCommand().ExecuteContext(ctrl.SetupSignalHandler()); err != nil {
		os.Exit(1)
	}
}

// newRootCommand builds the command tree. Every subcommand exits non-zero on
// failure, which fails the Job that runs it.
func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:          "camunda-operator-cli",
		Short:        "Command-line companion of the Camunda operator, run inside its Jobs",
		SilenceUsage: true,
	}
	root.AddCommand(newUploadCommand())
	root.AddCommand(newDeleteCommand())

	return root
}

// newUploadCommand streams one file to the backup bucket. Its whole
// interface is the UPLOAD_* environment that the dump Job renders.
func newUploadCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "upload",
		Short: "Upload a file to the backup bucket described by the UPLOAD_* environment",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return upload.Run(cmd.Context())
		},
	}
}

// newDeleteCommand removes one object from the backup bucket. The cleanup
// Job of a deleted backup runs it under the cluster ServiceAccount, so a
// bucket on workload identity is cleaned by the identity that wrote it. Its
// whole interface is the same UPLOAD_* environment; a missing key is
// success, so a retry is idempotent.
func newDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete",
		Short: "Delete an object from the backup bucket described by the UPLOAD_* environment",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return deleteobject.Run(cmd.Context())
		},
	}
}
