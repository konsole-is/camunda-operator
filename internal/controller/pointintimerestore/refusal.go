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
	"context"
	"fmt"
	"io"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// markerExportedPosition is the phrase that the Camunda restore application
// prints when no primary-storage checkpoint covers the exporter position that
// the restored database records. The application refuses in two forms, and
// both carry this phrase: a checkpoint whose last log position is before the
// exported position, and a checkpoint whose first log position is after it.
//
// The operator matches the phrase and nothing else. The rest of the line
// carries the positions of one run, and it reaches the user as a quotation.
const markerExportedPosition = "required exported position"

// maxRefusalLine bounds the log line that the failure message quotes. The
// whole message is bounded again before it reaches the status, and a bound
// here keeps the quotation readable next to the words around it.
const maxRefusalLine = 400

// maxLogLines and maxLogBytes bound one read of a restore Job log. The
// refusal is one line of a short log, and the operator reads the log inside a
// reconcile, so an unbounded read is not worth its memory.
const (
	maxLogLines = int64(2000)
	maxLogBytes = int64(256 * 1024)
)

// ReadJobLog reads the log of the pod that ran a restore Job. It reports an
// error when no pod is left, or when the API refuses the read. The caller
// treats every error as "the cause is not readable" and keeps the failure it
// already has.
type ReadJobLog func(ctx context.Context, job types.NamespacedName) (string, error)

// jobRefusal reports the one failure of a restore Job that the operator can
// name: the restore application found no primary-storage checkpoint that
// covers the exporter position of the restored database.
//
// It runs after a Job already failed, so it can never hold back or break a
// restore that works. It answers nil for a log it cannot read and for a log
// that carries no marker, and a restore that failed for another reason
// therefore keeps that reason.
func (r *Reconciler) jobRefusal(
	ctx context.Context,
	job *batchv1.Job,
	ordinal int32,
) *conditions.PreCheckFailure {
	if r.opts.ReadJobLog == nil {
		return nil
	}

	output, err := r.opts.ReadJobLog(ctx, client.ObjectKeyFromObject(job))
	if err != nil {
		return nil
	}

	line := refusalLine(output)
	if line == "" {
		return nil
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonExporterPositionNotCovered,
		Message: fmt.Sprintf(
			"the restore application of broker %d found no primary-storage checkpoint that covers "+
				"the exporter position of the restored database, so it restored no partition. It "+
				"reported: %q. Roll the database back to an earlier point, whose exporter position "+
				"lies inside a checkpoint, then create a new restore",
			ordinal, line,
		),
	}
}

// refusalLine returns the first line of a restore application log that
// reports a refused exporter position, or the empty string when the log
// carries none. The line reaches a status field that a user reads, so it is
// trimmed, bounded, and always valid UTF-8.
func refusalLine(output string) string {
	for line := range strings.SplitSeq(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.Contains(trimmed, markerExportedPosition) {
			continue
		}
		if len(trimmed) > maxRefusalLine {
			trimmed = strings.ToValidUTF8(trimmed[:maxRefusalLine], "")
		}

		return trimmed
	}

	return ""
}

// podLogReader reads the log of a restore Job through the pods/log
// subresource. A client.Client cannot serve it: the subresource answers a
// stream rather than an object, so the read needs a typed REST client.
//
// A restore Job never retries, so it owns one pod. A pod that the node
// evicted leaves a second one behind, and the newest pod is the one that ran
// the restore application last.
func podLogReader(pods corev1client.PodsGetter) ReadJobLog {
	return func(ctx context.Context, job types.NamespacedName) (string, error) {
		list, err := pods.Pods(job.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: batchv1.JobNameLabel + "=" + job.Name,
		})
		if err != nil {
			return "", fmt.Errorf("listing the pods of the restore Job %s: %w", job, err)
		}

		name := newestPod(list.Items)
		if name == "" {
			return "", fmt.Errorf("no pod of the restore Job %s is left to read", job)
		}

		lines, bytes := maxLogLines, maxLogBytes
		stream, err := pods.Pods(job.Namespace).GetLogs(name, &corev1.PodLogOptions{
			TailLines: &lines, LimitBytes: &bytes,
		}).Stream(ctx)
		if err != nil {
			return "", fmt.Errorf("reading the log of pod %s/%s: %w", job.Namespace, name, err)
		}
		defer func() { _ = stream.Close() }()

		output, err := io.ReadAll(stream)
		if err != nil {
			return "", fmt.Errorf("reading the log of pod %s/%s: %w", job.Namespace, name, err)
		}

		return string(output), nil
	}
}

// newestPod returns the name of the pod that started last, or the empty
// string for an empty list.
func newestPod(pods []corev1.Pod) string {
	name := ""
	var newest metav1.Time
	for i := range pods {
		created := pods[i].CreationTimestamp
		if name == "" || newest.Before(&created) {
			name, newest = pods[i].Name, created
		}
	}

	return name
}
