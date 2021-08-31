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


set -ex

echo "Install cstor-operators in 2.0.0"

kubectl create ns openebs

kubectl apply -f https://raw.githubusercontent.com/openebs/charts/gh-pages/versioned/2.0.0/cstor-operator.yaml \
    -f ./ci/upgrade/cstor/ndm-operator.yaml 
sleep 100

echo "Wait for cspc-operator to start"

kubectl wait --for=condition=available --timeout=550s deployment/cspc-operator -n openebs

echo "Create application with cStor volume on CSPC"

nodename=$(kubectl get nodes -o jsonpath='{.items[*].metadata.name}')
bdname=$(kubectl -n openebs get blockdevices -o jsonpath='{.items[?(@.spec.details.deviceType=="sparse")].metadata.name}')
sed "s|CSPCBD|$bdname|g" ./ci/upgrade/cstor/application.tmp.yaml | sed "s|NODENAME|$nodename|g" > ./ci/upgrade/cstor/application.yaml
kubectl apply -f ./ci/upgrade/cstor/application.yaml
sleep 10
kubectl wait --for=condition=available --timeout=200s deployment/percona

echo "Upgrade control plane to latest version"

sed "s|testimage|$TEST_IMAGE_TAG|g" ./ci/upgrade/cstor/cstor-operator.tmp.yaml | sed "s|testversion|$TEST_VERSION|g" | sed "s|imageorg|$IMAGE_ORG|g" > ./ci/upgrade/cstor/cstor-operator.yaml

kubectl delete csidriver cstor.csi.openebs.io

kubectl apply -f https://raw.githubusercontent.com/openebs/cstor-operators/master/deploy/crds/all_cstor_crds.yaml \
 -f https://raw.githubusercontent.com/openebs/cstor-operators/master/deploy/rbac.yaml \
 -f https://raw.githubusercontent.com/openebs/cstor-operators/master/deploy/csi-operator.yaml \
 -f ./ci/upgrade/cstor/cstor-operator.yaml -f https://raw.githubusercontent.com/openebs/cstor-operators/master/deploy/ndm-operator.yaml
sleep 10
kubectl wait --for=condition=available --timeout=300s deployment/cspc-operator -n openebs

kubectl apply -f ./ci/upgrade/upgradetaskCRD.yaml
