# limitations under the License.

#!/usr/bin/env bash

set -ex

echo "Install cstor-operators in 1.10.0"

kubectl create ns openebs

kubectl apply -f ./ci/upgrade/ndm-operator.yaml \
 -f https://raw.githubusercontent.com/openebs/charts/master/archive/1.10.x/csi-operator-1.10.0-ubuntu-18.04.yaml \
 -f https://raw.githubusercontent.com/openebs/charts/master/archive/1.10.x/cstor-operator-1.10.0.yaml 
sleep 100

echo "Wait for cspc-operator to start"

kubectl wait --for=condition=available --timeout=300s deployment/cspc-operator -n openebs

echo "Create application with cStor volume on CSPC"

nodename=$(kubectl get nodes -o jsonpath='{.items[*].metadata.name}')
bdname=$(kubectl -n openebs get blockdevices -o jsonpath='{.items[*].metadata.name}')
sed "s|CSPCBD|$bdname|g" ./ci/upgrade/application.tmp.yaml | sed "s|NODENAME|$nodename|g" > ./ci/upgrade/application.yaml
kubectl apply -f ./ci/upgrade/application.yaml
sleep 10
kubectl wait --for=condition=available --timeout=300s deployment/percona

echo "Upgrade control plane to latest version"

sed "s|testimage|$TEST_IMAGE_TAG|g" ./ci/upgrade/cstor-operator.tmp.yaml | sed "s|testversion|$TEST_VERSION|g" | sed "s|imageorg|$IMAGE_ORG|g" > ./ci/upgrade/cstor-operator.yaml

kubectl apply -f https://raw.githubusercontent.com/openebs/cstor-operators/master/deploy/csi-operator.yaml \
 -f ./ci/upgrade/cstor-operator.yaml
sleep 10
kubectl wait --for=condition=available --timeout=300s deployment/cspc-operator -n openebs
