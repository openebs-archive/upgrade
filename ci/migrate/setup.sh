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

wget https://raw.githubusercontent.com/openebs/charts/gh-pages/cstor-operator.yaml
sed -ie 's|value: "0"|value: "1"|g' cstor-operator.yaml
kubectl apply -f cstor-operator.yaml
sleep 5

sed "s|testimage|$TEST_IMAGE_TAG|g" ./ci/upgrade/cstor-operator.tmp.yaml | sed "s|testversion|$TEST_VERSION|g" | sed "s|imageorg|$IMAGE_ORG|g" > ./ci/migrate/cstor-operator.yaml
kubectl apply -f ./ci/migrate/cstor-operator.yaml
sleep 10

kubectl wait --for=condition=available --timeout=600s deployment/cspc-operator -n openebs