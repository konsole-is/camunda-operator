# Releasing

This page is for maintainers. It tells you how to publish a release of the operator.

A release has one version for everything it publishes. Release `0.2.0` publishes:

- The Git tag `0.2.0`, and the Go module tags `v0.2.0` and `api/v0.2.0`.
- The manager image `ghcr.io/konsole-is/camunda-operator:0.2.0`.
- The CLI image `ghcr.io/konsole-is/camunda-operator-cli:0.2.0`.
- The chart `oci://ghcr.io/konsole-is/charts/camunda-operator` at version `0.2.0`.
- The release assets `install.yaml`, `crds.yaml`, and `camunda-operator-0.2.0.tgz`.
- The docs site at <https://konsole-is.github.io/camunda-operator/>, for a release that is not a prerelease.

The tag is unprefixed SemVer, because a Helm chart version must be bare SemVer. The release workflow stops on a tag such as `v0.2.0`.

No secrets are necessary. The workflows use `GITHUB_TOKEN` and cosign keyless signing.

## Publish a release

1. Make sure that the CI on main is green.
2. In the **Actions** tab, run the **Prepare release** workflow from main. Enter the version, for example `0.2.0`.
3. Open the job summary of the run. Use its link to open the pull request.
4. Merge the pull request when CI passes.
5. Draft a new GitHub Release. Set the tag to `0.2.0` and the target to main.
6. Write the release notes, then publish the release.
7. Make sure that the **Release** workflow passes.

If the summary says that `go.mod` already pins the version, there is no pull request to merge. Continue at step 5.

The Prepare release workflow pins the version of the api module in the root `go.mod`. Users of the root module get this version of the api module. The Release workflow stops if the pin does not match the tag.

## What the Release workflow does

When you publish the release, `.github/workflows/release.yml` starts. It does these steps:

1. It pushes the Go module tags `vX.Y.Z` and `api/vX.Y.Z` at the release commit.
2. It builds the chart, sets the release version in it, and lints it.
3. It builds the manager and CLI images for amd64 and arm64, and pushes them to GHCR.
4. It pushes the chart and the Artifact Hub metadata to GHCR.
5. It signs the two images and the chart by digest with cosign.
6. It attaches `install.yaml`, `crds.yaml`, and the packaged chart to the release.

When these steps pass, a second job builds the docs site and deploys it to GitHub Pages. A prerelease does not change the docs site. The site therefore always shows the latest stable release.

Users verify the signatures as [Verify the signatures](docs/installation.md#verify-the-signatures) shows.

## If the Release workflow fails

If the cause is outside the code, for example a registry outage, run the failed jobs again from the workflow run. A run again uses the same release commit. A Go module tag that already points at that commit stays as it is. A push of an image or a chart replaces the earlier push.

If the cause is in the code, correct it on main. Then look for the Go module tags `vX.Y.Z` and `api/vX.Y.Z` on GitHub:

- If the Go module tags exist, do not move or delete them. The Go checksum database can already hold the checksum of that version, and a moved tag then breaks `go get` for each user. Delete the failed GitHub release, and release a new version, for example `0.2.1`.
- If the Go module tags do not exist, delete the release and its tag. Then publish the release again at the new commit.

## One-time setup

Do these steps one time. Do steps 1 and 2 before the first release, and the other steps after it publishes:

1. In the repository settings, under **Pages**, set the source to **GitHub Actions**.
2. In the repository settings, under **Environments**, open `github-pages`. Add a deployment rule for tags with the pattern `*`. Without this rule, the docs job cannot deploy, because the release runs on a tag.
3. In the GHCR package settings of the organization, make these packages public:
    - `camunda-operator`
    - `camunda-operator-cli`
    - `charts/camunda-operator`
4. In the [Artifact Hub control panel](https://artifacthub.io/control-panel), add a Helm chart repository. Use the name `camunda-operator` and the URL `oci://ghcr.io/konsole-is/charts/camunda-operator`.
5. Copy the repository ID from Artifact Hub into `repositoryID` in `.github/artifacthub-repo.yml`. Artifact Hub then shows the verified publisher badge after the next release.

Artifact Hub reads `.github/artifacthub-repo.yml` again only when the chart repository changes. A change to that file therefore takes effect at the next release.
