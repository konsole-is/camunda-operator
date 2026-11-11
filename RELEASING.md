# Releasing

This page is for maintainers. It tells you how to publish a release of the operator.

A release has one version for everything it publishes. The release tag is `v` and SemVer, for example `v0.2.0`. The release `v0.2.0` publishes:

- The release tag `v0.2.0`, which is also the Go tag of the root module.
- The Go tag `api/v0.2.0` of the api module, and the Go tag `conformance/v0.2.0` of the conformance module.
- The manager image `ghcr.io/konsole-is/camunda-operator:0.2.0`.
- The CLI image `ghcr.io/konsole-is/camunda-operator-cli:0.2.0`.
- The fleet manager image `ghcr.io/konsole-is/camunda-operator-fleet-manager:0.2.0`.
- The plugin conformance image `ghcr.io/konsole-is/camunda-operator-plugin-conformance:0.2.0`.
- The chart `oci://ghcr.io/konsole-is/charts/camunda-operator` at version `0.2.0`.
- The release assets `install.yaml`, `crds.yaml`, and `camunda-operator-0.2.0.tgz`.
- The release assets `plugin-conformance-<os>-<arch>`, one binary for each of `linux-amd64`, `linux-arm64`, `darwin-amd64`, and `darwin-arm64`.
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

If the summary says that `go.mod` and `conformance/go.mod` already pin the version, there is no pull request to merge. Continue at step 5.

The Prepare release workflow pins the api module in the root `go.mod` and in `conformance/go.mod`, and the conformance module in the root `go.mod`. Users of each module then get the modules of the same release. The Release workflow stops if a pin does not match the tag.

## Check a release with a dry run

You can check the Release workflow before you publish a release:

1. Run the **Prepare release** workflow from main with the release tag, for example `v0.2.0`. It pushes the branch `chore/release-v0.2.0`.
2. In the **Actions** tab, run the **Release** workflow by hand on the branch `chore/release-v0.2.0`. Enter the same tag.
3. Make sure that the run passes. Then merge the pull request of the branch, and publish the release.

A run by hand is a dry run. It builds and scans the images, builds the chart and the binaries, and checks the pins. It pushes nothing: no Go tag, image, chart, signature, release asset, or docs version. The job summary names the Go tags that the release pushes.

A dry run on main stops at the pin check until you merge the pull request of the Prepare release workflow.

## What the Release workflow does

When you publish the release, `.github/workflows/release.yml` starts. It does these steps:

1. It builds the manager, CLI, fleet manager, and plugin conformance images for amd64 and arm64, and scans each image with Trivy. It also starts the fleet manager image of each platform. It stops on a HIGH or CRITICAL finding that has a fix. The next steps start only when every scan passes.
2. It pushes the Go tags `api/vX.Y.Z` and `conformance/vX.Y.Z` at the release commit.
3. It builds the chart, sets the release version in it, and lints it.
4. It builds the manager, CLI, fleet manager, and plugin conformance images for amd64 and arm64, and pushes them to GHCR.
5. It pushes the chart and the Artifact Hub metadata to GHCR.
6. It signs the four images and the chart by digest with cosign.
7. It builds the `plugin-conformance` binaries. It attaches them, `install.yaml`, `crds.yaml`, and the packaged chart to the release.

When these steps pass, a second job deploys the docs version of the minor release, such as `0.2`, to the `gh-pages` branch. It moves the alias `latest` to that version, and the site root opens `latest`. A patch release replaces the docs of its minor. A prerelease does not change the docs site.

Separately, the **Docs** workflow deploys the version `dev` on each push to main that changes the docs. The README links to `dev`.

Users verify the signatures as [Verify the signatures](docs/installation.md#verify-the-signatures) shows.

## If the Release workflow fails

If the image scan stops the workflow, the workflow has not pushed a tag or an image. The job log names each finding and the version that fixes it. Delete the release and its tag `vX.Y.Z`. Fix the finding on main. For a Go module, Renovate opens a security pull request. Then do the steps of [Publish a release](#publish-a-release) again from step 2.

If the workflow stops because `go.mod` or `conformance/go.mod` does not pin a module at the release version, the Go module tags do not exist yet. Delete the release and its tag `vX.Y.Z`, because the Prepare release workflow rejects a version whose tag exists. Then do the steps of [Publish a release](#publish-a-release) again from step 2.

If the cause is outside the code, for example a registry outage, run the failed jobs again from the workflow run. A run again uses the same release commit. A Go module tag that already points at that commit stays as it is. A push of an image or a chart replaces the earlier push.

If the cause is in the code, correct it on main. Then look for the Go module tags `api/vX.Y.Z` and `conformance/vX.Y.Z` on GitHub:

- If one of the Go module tags exists, the versions are published. Do not move or delete a tag. The Go checksum database can already hold the checksum of that version, and a moved tag then breaks `go get` for each user. Delete the failed GitHub release, but keep its tag. Then release a new version, for example `v0.2.1`.
- If neither Go module tag exists, delete the release and its tag. Then publish the release again at the new commit.

## Vulnerability scans

The **Security** workflow scans the code and the images:

- On each pull request, on each push to main, every Monday, and when you run it by hand, it runs `govulncheck` on every Go module and Trivy on every image that it builds.
- Every Monday, and when you run it by hand, it also scans the amd64 and arm64 images of the latest release.

`govulncheck` fails its job on a known vulnerability that the code can reach. Trivy fails a job on a HIGH or CRITICAL finding that has a fix. The **Security** tab of the repository shows all Trivy findings under code scanning. If the weekly scan of the latest release fails, fix the finding on main, then publish a patch release.

Renovate opens a security pull request for a Go module only after the advisory is in the OSV database or in the GitHub Advisory Database. Until then, update the module yourself.

If a HIGH or CRITICAL finding has a fix that the operator cannot use yet, add its ID to a file `.trivyignore` at the root of the repository. Write a comment above the ID that says why, and add an end date, for example `CVE-2026-12345 exp:2026-12-31`. Trivy reads this file in each scan of the Security and Release workflows, and reports the finding again after the end date.

## One-time setup

Do these steps one time. Do steps 1 and 2 before the first release, and the other steps after it publishes:

1. In the repository settings, under **Pages**, set the source to **Deploy from a branch**, with the branch `gh-pages` and the folder `/ (root)`. The branch exists after the first run of the Docs workflow.
2. In the repository settings, under **Environments**, open `github-pages`. Add a deployment rule for the branch `gh-pages`. Without this rule, GitHub Pages cannot publish the branch.
3. In the GHCR package settings of the organization, make these packages public:
    - `camunda-operator`
    - `camunda-operator-cli`
    - `camunda-operator-fleet-manager`
    - `camunda-operator-plugin-conformance`
    - `charts/camunda-operator`

    The package `camunda-operator-plugin-conformance` exists after the first release that publishes it. Make it public after that release.
4. In the [Artifact Hub control panel](https://artifacthub.io/control-panel), add a Helm chart repository. Use the name `camunda-operator` and the URL `oci://ghcr.io/konsole-is/charts/camunda-operator`.
5. Copy the repository ID from Artifact Hub into `repositoryID` in `.github/artifacthub-repo.yml`. Artifact Hub then shows the verified publisher badge after the next release.

Artifact Hub reads `.github/artifacthub-repo.yml` again only when the chart repository changes. A change to that file therefore takes effect at the next release.
