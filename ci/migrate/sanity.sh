# limitations under the License.

#!/usr/bin/env bash

make migrate-image.amd64

# setup openebs & cstor v1 for migration 
./ci/migrate/setup.sh || exit 1
# run migration tests
./ci/migrate/test.sh 
if [[ $? != 0 ]]; then
  kubectl logs --tail=50 -l job-name=migrate-pool -n openebs
  kubectl logs --tail=50 -l job-name=migrate-volume -n openebs
  exit 1
fi

rm ./ci/migrate/volume.yaml ./ci/migrate/application.yaml