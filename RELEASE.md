# Release Process
upgrade follows a monthly release cadence. The scope of the release is determined by contributor availability. The scope is published in the [Release Tracker Projects](https://github.com/orgs/openebs/projects).

## Release Candidate Verification Checklist

Every release has release candidate builds that are created starting from the third week into the release. These release candidate builds help to freeze the scope and maintain the quality of the release. The release candidate builds will go through:
- Platform Verification
- Regression and Feature Verification Automated tests.
- Exploratory testing by QA engineers
- Strict security scanners on the container images
- Upgrade from previous releases
- Beta testing by users on issues that they are interested in.
- Dogfooding on OpenEBS workload and e2e infrastructure clusters.

If any issues are found during the above stages, they are fixed and a new release candidate builds are generated.

Once all the above tests are completed, a main release tagged image is published.

## Release Tagging

upgrade is released as a set of container images with a versioned tag.

Before creating a release, the repo owner needs to create a separate branch from the active branch, which is `master`. Name of the branch should follow the naming convention of `v.1.12.x` if the release is for v1.12.0.

Once the release branch is created, changelog from `changelogs/unreleased` needs to be moved to release specific folder `changelogs/v1.12.x`, if release branch is `v2.0.x` then folder will be `changelogs/v2.0.x`.

The format of the release tag is either "Release-Name-RC1" or "Release-Name" depending on whether the tag is a release candidate or a release. (Example: v1.12.0-RC1 is a GitHub release tag for the upgrade release build. v1.12.0 is the release tag that is created after the release criteria are satisfied by the upgrade builds.)

Once the release is triggered, Travis build process has to be monitored. Once Travis build is passed images are pushed to docker hub and quay.io. Images can be verified by going through docker hub and quay.io. Also the images shouldn't have any high-level vulnerabilities.

Images for the different components are published at the following location:

- Upgrade <br />
    https://quay.io/repository/openebs/upgrade-amd64?tab=tags <br />
    https://hub.docker.com/r/openebs/upgrade-amd64/tags <br />

- Migrate <br />
    https://quay.io/repository/openebs/migrate-amd64?tab=tags <br />
    https://hub.docker.com/r/openebs/migrate-amd64/tags <br />


Once a release is created, update the release description with the changelog mentioned in `changelog/v1.12.x`. Once the changelogs are updated in the release, the repo owner needs to create a PR to `master` with the following details:
1. update the changelog from `changelog/v1.12.x` to `upgrade/CHANGELOG.md`
2. If a release is not an RC tag then PR should include the changes to remove `changelog/v1.12.x` folder.
3. If a release is an RC tag then PR should include the changes to remove the changelog from `changelog/v1.12.x` which are already mentioned in `upgrade/CHANGELOG.md` as part of step number 1.