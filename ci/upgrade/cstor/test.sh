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

echo "Upgrading CSPC pool"

sed "s|testimage|$TEST_IMAGE_TAG|g" ./ci/upgrade/cstor/pool.tmp.yaml | sed "s|testversion|$TEST_VERSION|g" | sed "s|imageorg|$IMAGE_ORG|g" > ./ci/upgrade/cstor/pool.yaml
kubectl apply -f ./ci/upgrade/cstor/pool.yaml
sleep 5
kubectl wait --for=condition=complete job/upgrade-pool -n openebs --timeout=550s
kubectl logs --tail=50 -l job-name=upgrade-pool -n openebs

echo "Upgrading CSI volume"

pvname=$(kubectl get pvc demo-csi-vol-claim -o jsonpath="{.spec.volumeName}")
sed "s|PVNAME|$pvname|g" ./ci/upgrade/cstor/volume.tmp.yaml | sed "s|testimage|$TEST_IMAGE_TAG|g" | sed "s|testversion|$TEST_VERSION|g" | sed "s|imageorg|$IMAGE_ORG|g" > ./ci/upgrade/cstor/volume.yaml
kubectl apply -f ./ci/upgrade/cstor/volume.yaml
sleep 5
kubectl wait --for=condition=complete job/upgrade-volume -n openebs --timeout=550s
kubectl logs --tail=50 -l job-name=upgrade-volume -n openebs
