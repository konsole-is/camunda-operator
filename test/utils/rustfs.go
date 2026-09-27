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

// AWSCLIImage is the pinned aws-cli image of the helper pods that read the
// bucket. It is the image of the bucket Job of testdata/rustfs.yaml.
// renovate: datasource=docker depName=amazon/aws-cli
const AWSCLIImage = "amazon/aws-cli:2.37.4"

// The names that testdata/rustfs.yaml creates. The manifest is the one
// definition. These constants mirror it, so a spec repeats no string.
const (
	// RustFSDeployment and RustFSService are the server and the Service that
	// serves its S3 API.
	RustFSDeployment = "rustfs"
	RustFSService    = "rustfs"
	// RustFSPort is the S3 API port of the Service.
	RustFSPort = 9000
	// RustFSBucket is the bucket that the manifest creates. Every backup of
	// the suite writes into it.
	RustFSBucket = "camunda-backup"
	// RustFSCredentialsSecret holds the access-key pair of the store, under
	// RustFSAccessKeyIDKey and RustFSSecretAccessKeyKey. It is also the root
	// account of the server.
	RustFSCredentialsSecret  = "rustfs-credentials"
	RustFSAccessKeyIDKey     = "accessKeyId"
	RustFSSecretAccessKeyKey = "secretAccessKey"
	// RustFSAccessKeyID and RustFSSecretAccessKey are the values of that pair.
	// A bucket contract names a Secret of its own namespace, so each flow
	// creates the pair next to its contract.
	RustFSAccessKeyID     = "camundabackup"
	RustFSSecretAccessKey = "camunda-backup-secret"
	// rustfsBucketJob creates RustFSBucket once the server answers.
	rustfsBucketJob = "rustfs-bucket"
	// rustfsServerSelector and rustfsBucketJobSelector select the pods of
	// RustFSDeployment and rustfsBucketJob. dumpInstallDiagnostics describes
	// the pods these match when a wait on the two fails.
	rustfsServerSelector    = "app=" + RustFSDeployment
	rustfsBucketJobSelector = "batch.kubernetes.io/job-name=" + rustfsBucketJob
	// rustfsManifest is the manifest of all of the above, relative to the
	// project directory that Run works in.
	rustfsManifest = "test/e2e/testdata/rustfs.yaml"
)

// RustFSEndpoint returns the URL of the S3 API of the RustFS server that runs
// in namespace. It is the endpoint that an ObjectStorageConfig of the suite
// carries, and consumers in other namespaces reach it through the same URL.
func RustFSEndpoint(namespace string) string {
	return "http://" + RustFSService + "." + namespace + ".svc:" + strconv.Itoa(RustFSPort)
}

// InstallRustFS applies the RustFS manifest into namespace and returns once
// the server answers and RustFSBucket exists.
//
// Every call creates the bucket again. The server keeps its data in an
// emptyDir, so a restarted pod comes back without the bucket. The bootstrap
// Job of the first call stays Completed over that empty server. A caller that
// waits on that old Job gets success against a bucket that is gone.
//
// This function therefore deletes the Job first. The delete cascades to its
// pods and waits for them, so the new Job never adopts one.
//
// When the wait on the rollout or on the bucket Job fails, it writes the pod
// descriptions and the events of namespace to the Ginkgo writer before it
// returns the error.
func InstallRustFS(namespace string) error {
	if _, err := Kubectl(
		"delete", "job/"+rustfsBucketJob,
		"-n", namespace, "--ignore-not-found", "--cascade=foreground", "--wait=true",
	); err != nil {
		return err
	}

	if _, err := Kubectl("apply", "-n", namespace, "-f", rustfsManifest); err != nil {
		return err
	}

	if _, err := Kubectl(
		"rollout", "status", "deployment/"+RustFSDeployment,
		"-n", namespace, "--timeout", "5m",
	); err != nil {
		dumpInstallDiagnostics(namespace, rustfsServerSelector)
		return err
	}

	if _, err := Kubectl(
		"wait", "--for=condition=complete", "job/"+rustfsBucketJob,
		"-n", namespace, "--timeout", "5m",
	); err != nil {
		dumpInstallDiagnostics(namespace, rustfsBucketJobSelector)
		return err
	}

	return nil
}

// RustFSObject is one object of the bucket. Key is the key under the bucket,
// without the bucket name.
type RustFSObject struct {
	Key  string `json:"key"`
	Size int64  `json:"size"`
}

// RustFSObjects returns every object of RustFSBucket, read through a helper
// pod in namespace. The caller selects the keys that it needs.
//
// A listing that the store refuses is an error, never an empty result. An
// empty result is what the deletion specs assert, so a failed listing must
// not pass for one.
func RustFSObjects(namespace string, timeout time.Duration) ([]RustFSObject, error) {
	out, err := RunPod(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "aws-ls-" + utilrand.String(5),
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "aws",
				Image: AWSCLIImage,
				// aws exits non-zero when the listing fails, so RunPod turns a
				// refused listing into an error.
				Args: []string{
					"s3api", "list-objects-v2",
					"--bucket", RustFSBucket,
					"--query", "Contents[].{key:Key,size:Size}",
					"--output", "json",
				},
				Env: []corev1.EnvVar{
					SecretEnv("AWS_ACCESS_KEY_ID", RustFSCredentialsSecret, RustFSAccessKeyIDKey),
					SecretEnv("AWS_SECRET_ACCESS_KEY", RustFSCredentialsSecret, RustFSSecretAccessKeyKey),
					{Name: "AWS_DEFAULT_REGION", Value: "us-east-1"},
					{Name: "AWS_ENDPOINT_URL", Value: "http://" + RustFSService + ":" + strconv.Itoa(RustFSPort)},
				},
			}},
		},
	}, timeout)
	if err != nil {
		return nil, err
	}

	return parseRustFSListing(out)
}

// parseRustFSListing decodes the output of list-objects-v2 under the query of
// RustFSObjects. aws prints null for a bucket that holds no object.
func parseRustFSListing(out string) ([]RustFSObject, error) {
	var objects []RustFSObject
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &objects); err != nil {
		return nil, fmt.Errorf("decoding the bucket listing %q: %w", out, err)
	}

	return objects, nil
}

// RustFSObjectsWithPrefix returns the objects of RustFSBucket whose key starts
// with prefix.
func RustFSObjectsWithPrefix(namespace, prefix string, timeout time.Duration) ([]RustFSObject, error) {
	objects, err := RustFSObjects(namespace, timeout)
	if err != nil {
		return nil, err
	}

	var matched []RustFSObject
	for _, object := range objects {
		if strings.HasPrefix(object.Key, prefix) {
			matched = append(matched, object)
		}
	}

	return matched, nil
}
