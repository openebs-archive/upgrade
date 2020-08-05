# limitations under the License.

#!/usr/bin/env bash

# To enable dev upgardes in travis
make upgrade-image.amd64

# To test the sanity in different versioned branches 
# and travis tags, get the travis version and corresponding
# image tags
# Determine the current branch
CURRENT_BRANCH=""
if [ -z ${TRAVIS_BRANCH} ];
then
  CURRENT_BRANCH=$(git branch | grep \* | cut -d ' ' -f2)
else
  CURRENT_BRANCH=${TRAVIS_BRANCH}
fi

TEST_IMAGE_TAG="${CURRENT_BRANCH}-ci"
if [ ${CURRENT_BRANCH} = "master" ]; then
  TEST_IMAGE_TAG="ci"
fi
TEST_VERSION="${CURRENT_BRANCH}-dev"

if [ -n "$TRAVIS_TAG" ]; then
    # Trim the `v` from the TRAVIS_TAG if it exists
    # Example: v1.10.0 maps to 1.10.0
    # Example: 1.10.0 maps to 1.10.0
    # Example: v1.10.0-custom maps to 1.10.0-custom
    TEST_IMAGE_TAG="${TRAVIS_TAG#v}"
    TEST_VERSION="${TRAVIS_TAG#v}"
fi

export TEST_IMAGE_TAG=${TEST_IMAGE_TAG#v}
export TEST_VERSION=${TEST_VERSION#v}

# setup openebs & cstor v1 for migration 
./ci/upgrade/setup.sh || exit 1
# run migration tests
./ci/upgrade/test.sh || exit 1

rm ./ci/upgrade/volume.yaml ./ci/upgrade/application.yaml