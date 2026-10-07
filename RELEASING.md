# Releasing

This page is for maintainers. It tells you how to publish a release of the operator.

A release has one version for everything it publishes. The release tag is `v` and SemVer, for example `v0.2.0`. The release `v0.2.0` publishes:

- The release tag `v0.2.0`, which is also the Go tag of the root module, and the Go tag `api/v0.2.0` of the api module.
- The manager image `ghcr.io/konsole-is/camunda-operator:0.2.0`.
- The CLI image `ghcr.io/konsole-is/camunda-operator-cli:0.2.0`.
- The chart `oci://ghcr.io/konsole-is/charts/camunda-operator` at version `0.2.0`.
- The release assets `install.yaml`, `crds.yaml`, and `camunda-operator-0.2.0.tgz`.
- The docs version `0.2` at <https://konsole-is.github.io/camunda-operator/>, with the alias `latest`, for a release that is not a prerelease.

The chart, the images, and the chart file use the version without the `v`, because a Helm chart version must be bare SemVer. The release workflow stops on a tag without the `v`, such as `0.2.0`.

No secrets are necessary. The workflows use `GITHUB_TOKEN` and cosign keyless signing.

## Publish a release

1. Make sure that the CI on main is green.
2. In the **Actions** tab, run the **Prepare release** workflow from main. Enter the release tag, for example `v0.2.0`.
3. Open the job summary of the run. Use its link to open the pull request.
4. Merge the pull request when CI passes.
5. Draft a new GitHub Release. Set the tag to `v0.2.0` and the target to main.
6. Write the release notes, then publish the release.
7. Make sure that the **Release** workflow passes.

If the summary says that `go.mod` already pins the version, there is no pull request to merge. Continue at step 5.

The Prepare release workflow pins the version of the api module in the root `go.mod`. Users of the root module get this version of the api module. The Release workflow stops if the pin does not match the tag.

## What the Release workflow does

When you publish the release, `.github/workflows/release.yml` starts. It does these steps:

1. It builds the manager and CLI images for amd64 and scans them with Trivy. It stops on a HIGH or CRITICAL finding that has a fix.
2. It pushes the Go tag `api/vX.Y.Z` of the api module at the release commit.
3. It builds the chart, sets the release version in it, and lints it.
4. It builds the manager and CLI images for amd64 and arm64, and pushes them to GHCR.
5. It pushes the chart and the Artifact Hub metadata to GHCR.
6. It signs the two images and the chart by digest with cosign.
7. It attaches `install.yaml`, `crds.yaml`, and the packaged chart to the release.

When these steps pass, a second job deploys the docs version of the minor release, such as `0.2`, to the `gh-pages` branch. It moves the alias `latest` to that version, and the site root opens `latest`. A patch release replaces the docs of its minor. A prerelease does not change the docs site.

Separately, the **Docs** workflow deploys the version `dev` on each push to main that changes the docs. The README links to `dev`.

Users verify the signatures as [Verify the signatures](docs/installation.md#verify-the-signatures) shows.

## If the Release workflow fails

If the image scan stops the workflow, the workflow has not pushed a tag or an image. The job log names each finding and the version that fixes it. Delete the release and its tag `vX.Y.Z`. Fix the finding on main. For a Go module, Renovate opens a security pull request. Then do the steps of [Publish a release](#publish-a-release) again from step 2.

If the workflow stops because `go.mod` does not pin the api module at the release version, the api module tag does not exist yet. Delete the release and its tag `vX.Y.Z`, because the Prepare release workflow rejects a version whose tag exists. Then do the steps of [Publish a release](#publish-a-release) again from step 2.

If the cause is outside the code, for example a registry outage, run the failed jobs again from the workflow run. A run again uses the same release commit. An api module tag that already points at that commit stays as it is. A push of an image or a chart replaces the earlier push.

If the cause is in the code, correct it on main. Then look for the api module tag `api/vX.Y.Z` on GitHub:

- If the api module tag exists, the release tag `vX.Y.Z` is a published Go version too. Do not move or delete either tag. The Go checksum database can already hold the checksum of that version, and a moved tag then breaks `go get` for each user. Delete the failed GitHub release, but keep its tag. Then release a new version, for example `v0.2.1`.
- If the api module tag does not exist, delete the release and its tag. Then publish the release again at the new commit.

## Vulnerability scans

The **Security** workflow scans the code and the images:

- On each pull request and each push to main, it runs `govulncheck` on both Go modules and Trivy on the manager and CLI images that it builds.
- Every Monday, and when you run it by hand, it also scans the images of the latest release.

Trivy fails a job on a HIGH or CRITICAL finding that has a fix. The **Security** tab of the repository shows all Trivy findings under code scanning. If the weekly scan of the latest release fails, fix the finding on main, then publish a patch release.

## One-time setup

Do these steps one time. Do steps 1 and 2 before the first release, and the other steps after it publishes:

1. In the repository settings, under **Pages**, set the source to **Deploy from a branch**, with the branch `gh-pages` and the folder `/ (root)`. The branch exists after the first run of the Docs workflow.
2. In the repository settings, under **Environments**, open `github-pages`. Add a deployment rule for the branch `gh-pages`. Without this rule, GitHub Pages cannot publish the branch.
3. In the GHCR package settings of the organization, make these packages public:
    - `camunda-operator`
    - `camunda-operator-cli`
    - `charts/camunda-operator`
4. In the [Artifact Hub control panel](https://artifacthub.io/control-panel), add a Helm chart repository. Use the name `camunda-operator` and the URL `oci://ghcr.io/konsole-is/charts/camunda-operator`.
5. Copy the repository ID from Artifact Hub into `repositoryID` in `.github/artifacthub-repo.yml`. Artifact Hub then shows the verified publisher badge after the next release.

Artifact Hub reads `.github/artifacthub-repo.yml` again only when the chart repository changes. A change to that file therefore takes effect at the next release.
