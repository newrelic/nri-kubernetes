# Releasing a new version

- Run the `release.sh` script to update the version number in the code and 
  manifests files, commit and push the changes.
  - This script could fail to run, because of major differences in `sed` across systems. If this happens, manually changing the entries found in `release.sh` also works.
- Create a branch called `release/X.Y.Z` where `X.Y.Z` is the [Semantic Version](https://semver.org/#semantic-versioning-specification-semver) to
  release. This will trigger the Jenkins job that pushes the image to
  be released to quay. This is done in the `Jenkinsfile` jobs. Make sure the PR
  job finishes successfully. This branch doesn't need to be merged.
- Create the Github release.
- Run the k8s-integration-release Jenkins job.
- Update the release notes under the [On-Host Integrations Release Notes](https://docs.newrelic.com/docs/release-notes/platform-release-notes).