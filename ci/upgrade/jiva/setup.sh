#!/usr/bin/env bash

# Copyright © 2021 The OpenEBS Authors
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

echo "Install jiva-operator in 2.7.0"

kubectl create ns openebs

kubectl apply -f https://raw.githubusercontent.com/openebs/charts/gh-pages/versioned/2.7.0/jiva-operator.yaml \
 -f https://raw.githubusercontent.com/openebs/charts/gh-pages/versioned/2.7.0/openebs-operator.yaml 
sleep 100

echo "Wait for jiva-operator to start"

kubectl wait --for=condition=available --timeout=550s deployment/jiva-operator -n openebs

echo "Create application with jiva volume on openebs-hostpath"

kubectl apply -f ./ci/upgrade/jiva/application.yaml
sleep 10
kubectl wait --for=condition=available --timeout=200s deployment/fio

echo "Upgrade control plane to latest version"

sed "s|testimage|$TEST_IMAGE_TAG|g" ./ci/upgrade/jiva/jiva-operator.tmp.yaml | sed "s|testversion|$TEST_VERSION|g" | sed "s|imageorg|$IMAGE_ORG|g" > ./ci/upgrade/jiva/jiva-operator.yaml

kubectl apply -f https://raw.githubusercontent.com/openebs/jiva-operator/develop/deploy/jiva-operator.yaml \
 -f ./ci/upgrade/jiva/jiva-operator.yaml

sleep 50
kubectl wait --for=condition=available --timeout=300s deployment/jiva-operator -n openebs

kubectl apply -f ./ci/upgrade/upgradetaskCRD.yaml
