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

package utils

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

// MinIOClientImage is the pinned mc image of the helper pods that read the
// bucket. It carries no shell, so every call passes arguments to mc itself.
const MinIOClientImage = "minio/mc:RELEASE.2025-08-13T08-35-41Z"

// The names that testdata/minio.yaml creates. The manifest is the one
// definition. These constants mirror it, so a spec repeats no string.
const (
	// MinIODeployment and MinIOService are the server and the Service that
	// serves its S3 API.
	MinIODeployment = "minio"
	MinIOService    = "minio"
	// MinIOPort is the S3 API port of the Service.
	MinIOPort = 9000
	// MinIOBucket is the bucket that the manifest creates. Every backup of
	// the suite writes into it.
	MinIOBucket = "camunda-backup"
	// MinIOCredentialsSecret holds the access-key pair of the store, under
	// MinIOAccessKeyIDKey and MinIOSecretAccessKeyKey. It is also the root
	// account of the server.
	MinIOCredentialsSecret  = "minio-credentials"
	MinIOAccessKeyIDKey     = "accessKeyId"
	MinIOSecretAccessKeyKey = "secretAccessKey"
	// MinIOAccessKeyID and MinIOSecretAccessKey are the values of that pair.
	// A bucket contract names a Secret of its own namespace, so each flow
	// creates the pair next to its contract.
	MinIOAccessKeyID     = "camundabackup"
	MinIOSecretAccessKey = "camunda-backup-secret"
	// minioBucketJob creates MinIOBucket once the server answers.
	minioBucketJob = "minio-bucket"
	// minioManifest is the manifest of all of the above, relative to the
	// project directory that Run works in.
	minioManifest = "test/e2e/testdata/minio.yaml"
	// mcConfigDir is where mc keeps its configuration in the helper pod. The
	// image has no writable home, so the pod mounts an emptyDir here.
	mcConfigDir = "/mc"
)

// MinIOEndpoint returns the URL of the S3 API of the MinIO server that runs
// in namespace. It is the endpoint that an ObjectStorageConfig of the suite
// carries, and consumers in other namespaces reach it through the same URL.
func MinIOEndpoint(namespace string) string {
	return "http://" + MinIOService + "." + namespace + ".svc:" + strconv.Itoa(MinIOPort)
}

// InstallMinIO applies the MinIO manifest into namespace and returns once the
// server answers and MinIOBucket exists.
//
// Every call creates the bucket again. The server keeps its data in an
// emptyDir, so a restarted pod comes back without the bucket. The bootstrap
// Job of the first call stays Completed over that empty server. A caller that
// waits on that old Job gets success against a bucket that is gone.
//
// This function therefore deletes the Job first. The delete cascades to its
// pods and waits for them, so the new Job never adopts one.
func InstallMinIO(namespace string) error {
	if _, err := Kubectl(
		"delete", "job/"+minioBucketJob,
		"-n", namespace, "--ignore-not-found", "--cascade=foreground", "--wait=true",
	); err != nil {
		return err
	}

	if _, err := Kubectl("apply", "-n", namespace, "-f", minioManifest); err != nil {
		return err
	}

	if _, err := Kubectl(
		"rollout", "status", "deployment/"+MinIODeployment,
		"-n", namespace, "--timeout", "5m",
	); err != nil {
		return err
	}

	_, err := Kubectl(
		"wait", "--for=condition=complete", "job/"+minioBucketJob,
		"-n", namespace, "--timeout", "5m",
	)
	return err
}

// MinIOObject is one object of the bucket, as mc reports it. Key is the key
// under the bucket, without the bucket name.
type MinIOObject struct {
	Key  string `json:"key"`
	Size int64  `json:"size"`
}

// MinIOObjects returns every object of MinIOBucket, read through a helper pod
// in namespace. The caller selects the keys that it needs.
//
// The whole bucket is listed, not a prefix. mc fails on a prefix that holds
// nothing, and an emptied prefix is what the deletion specs assert.
//
// The pod reads the credentials from MinIOCredentialsSecret through
// MC_HOST_local. The kubelet builds that value from the two variables before
// it, so no credential reaches the pod spec or the output of the suite.
func MinIOObjects(namespace string, timeout time.Duration) ([]MinIOObject, error) {
	out, err := RunPod(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mc-ls-" + utilrand.String(5),
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "mc",
				Image: MinIOClientImage,
				Args: []string{
					"--config-dir", mcConfigDir,
					"ls", "--recursive", "--json", "local/" + MinIOBucket,
				},
				Env: []corev1.EnvVar{
					SecretEnv("MINIO_ACCESS_KEY_ID", MinIOCredentialsSecret, MinIOAccessKeyIDKey),
					SecretEnv("MINIO_SECRET_ACCESS_KEY", MinIOCredentialsSecret, MinIOSecretAccessKeyKey),
					{
						Name: "MC_HOST_local",
						Value: "http://$(MINIO_ACCESS_KEY_ID):$(MINIO_SECRET_ACCESS_KEY)@" +
							MinIOService + ":" + strconv.Itoa(MinIOPort),
					},
				},
				VolumeMounts: []corev1.VolumeMount{{Name: "config", MountPath: mcConfigDir}},
			}},
			Volumes: []corev1.Volume{{
				Name:         "config",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
			}},
		},
	}, timeout)
	if err != nil {
		return nil, err
	}

	return parseMinIOListing(out)
}

// parseMinIOListing turns the JSON lines of mc ls --json into objects. mc
// reports one entry per line, and a listing of an empty bucket has no line at
// all. An entry that is not a file is skipped, so a prefix cannot become a
// phantom object.
//
// An entry that reports anything other than success is an error. The listing
// is then incomplete, and an incomplete listing that reads as "no objects"
// passes a deletion assertion for the wrong reason.
func parseMinIOListing(out string) ([]MinIOObject, error) {
	var objects []MinIOObject
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry struct {
			Status string `json:"status"`
			Type   string `json:"type"`
			MinIOObject
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("decoding the mc listing line %q: %w", line, err)
		}

		if entry.Status != "success" {
			return nil, fmt.Errorf("the mc listing reports %q", line)
		}
		if entry.Type != "file" {
			continue
		}

		objects = append(objects, entry.MinIOObject)
	}

	return objects, nil
}

// MinIOObjectsWithPrefix returns the objects of MinIOBucket whose key starts
// with prefix.
func MinIOObjectsWithPrefix(namespace, prefix string, timeout time.Duration) ([]MinIOObject, error) {
	objects, err := MinIOObjects(namespace, timeout)
	if err != nil {
		return nil, err
	}

	var matched []MinIOObject
	for _, object := range objects {
		if strings.HasPrefix(object.Key, prefix) {
			matched = append(matched, object)
		}
	}

	return matched, nil
}
