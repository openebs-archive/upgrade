# limitations under the License.

#!/usr/bin/env bash

set -ex

echo "Upgrading CSPC pool"

sed "s|testimage|$TEST_IMAGE_TAG|g" ./ci/upgrade/pool.tmp.yaml | sed "s|testversion|$TEST_VERSION|g" > ./ci/upgrade/pool.yaml
kubectl apply -f ./ci/upgrade/pool.yaml
sleep 5
kubectl wait --for=condition=complete job/upgrade-pool -n openebs --timeout=800s
kubectl logs --tail=50 -l job-name=upgrade-pool -n openebs

echo "Upgrading CSI volume"

pvname=$(kubectl get pvc demo-csi-vol-claim -o jsonpath="{.spec.volumeName}")
sed "s|PVNAME|$pvname|g" ./ci/upgrade/volume.tmp.yaml | sed "s|testimage|$TEST_IMAGE_TAG|g" | sed "s|testversion|$TEST_VERSION|g" > ./ci/upgrade/volume.yaml
kubectl apply -f ./ci/upgrade/volume.yaml
sleep 5
kubectl wait --for=condition=complete job/upgrade-volume -n openebs --timeout=800s
kubectl logs --tail=50 -l job-name=upgrade-volume -n openebs
