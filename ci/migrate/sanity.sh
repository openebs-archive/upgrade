# limitations under the License.

#!/usr/bin/env bash

make migrate-image.amd64

# setup openebs & cstor v1 for migration 
./ci/migrate/setup.sh || exit 1
# run migration tests
./ci/migrate/test.sh || exit 1

rm ./ci/migrate/volume.yaml ./ci/migrate/application.yaml