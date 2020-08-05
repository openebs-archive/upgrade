# limitations under the License.

#!/usr/bin/env bash

set -ex

echo "Install openebs-operator 1.11.0"

kubectl apply -f ./ci/migrate/openebs-operator.yaml
sleep 5

echo "Wait for maya-apiserver to start"

kubectl wait --for=condition=available --timeout=600s deployment/maya-apiserver -n openebs

echo "Label the node"

kubectl label nodes --all nodetype=storage

echo "Create application with cStor volume on SPC"

bdname=$(kubectl -n openebs get blockdevices -o jsonpath='{.items[*].metadata.name}')
sed "s/SPCBD/$bdname/" ./ci/migrate/application.tmp.yaml > ./ci/migrate/application.yaml
kubectl apply -f ./ci/migrate/application.yaml
sleep 5
kubectl wait --for=condition=Ready pod -l lkey=lvalue --timeout=600s

echo "Install cstor & csi operators"

kubectl apply -f https://raw.githubusercontent.com/openebs/charts/gh-pages/csi-operator-ubuntu-18.04.yaml \
 -f https://raw.githubusercontent.com/openebs/charts/gh-pages/cstor-operator.yaml
sleep 5
kubectl wait --for=condition=available --timeout=600s deployment/cspc-operator -n openebs