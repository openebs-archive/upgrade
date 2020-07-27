# limitations under the License.

#!/usr/bin/env bash

make migrate-image.amd64

# setup openebs & cstor v1 for migration 
./ci/migrate/setup.sh
# run migration tests
./ci/migrate/test.sh

rm ./ci/migrate/volume.yaml ./ci/migrate/application.yaml