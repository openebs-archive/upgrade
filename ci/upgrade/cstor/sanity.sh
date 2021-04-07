#!/usr/bin/env bash

# Copyright © 2020 The OpenEBS Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


set -x

sudo modprobe iscsi_tcp

# To test the sanity in different customized
# image prefixes
if [[ "${IMAGE_ORG}" == "" ]]; then
  IMAGE_ORG="openebs";
  export IMAGE_ORG;
fi

# To test the sanity in different versioned branches 
# and release tags, get the release version and corresponding
# image tags
# Determine the current branch
CURRENT_BRANCH=""
if [ -z "${BRANCH}" ];
then
  CURRENT_BRANCH=$(git branch | grep \* | cut -d ' ' -f2)
else
  CURRENT_BRANCH="${BRANCH}"
fi

TEST_IMAGE_TAG="${CURRENT_BRANCH}-ci"
if [ "${CURRENT_BRANCH}" = "master" ]; then
  TEST_IMAGE_TAG="ci"
fi
TEST_VERSION="${CURRENT_BRANCH}-dev"

if [ -n "$RELEASE_TAG" ]; then
    # Trim the `v` from the RELEASE_TAG if it exists
    # Example: v1.10.0 maps to 1.10.0
    # Example: 1.10.0 maps to 1.10.0
    # Example: v1.10.0-custom maps to 1.10.0-custom
    TEST_IMAGE_TAG="${RELEASE_TAG#v}"
    TEST_VERSION="${RELEASE_TAG#v}"
fi

export TEST_IMAGE_TAG="${TEST_IMAGE_TAG#v}"
export TEST_VERSION="${TEST_VERSION#v}"

echo "Testing upgrade for org: $IMAGE_ORG version: $TEST_VERSION imagetag: $TEST_IMAGE_TAG"

# setup openebs & cstor v1 for migration 
./ci/upgrade/cstor/setup.sh || exit 1
# run migration tests
./ci/upgrade/cstor/test.sh 
if [ $? != 0 ]; then
  kubectl logs -l job-name=upgrade-pool -n openebs
  kubectl logs -l job-name=upgrade-volume -n openebs
  kubectl describe pods -n openebs
  exit 1
fi

rm ./ci/upgrade/cstor/volume.yaml ./ci/upgrade/cstor/application.yaml