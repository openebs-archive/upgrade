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

#!/usr/bin/env bash

set -ex

echo "Scaling down application before migration"

kubectl scale statefulset busybox --replicas=0
kubectl wait --for=delete pod -l lkey=lvalue --timeout=600s 

echo "Migrating pool to cspc"

kubectl apply -f ./ci/migrate/pool.yaml
sleep 5
kubectl wait --for=condition=complete job/migrate-pool -n openebs --timeout=550s
kubectl logs --tail=50 -l job-name=migrate-pool -n openebs

echo "Migrating extetnal volume to csi volume"

pvname=$(kubectl get pvc testclaim-busybox-0 -o jsonpath="{.spec.volumeName}")
sed "s/PVNAME/$pvname/" ./ci/migrate/volume.tmp.yaml > ./ci/migrate/volume.yaml
kubectl apply -f ./ci/migrate/volume.yaml
sleep 5
kubectl wait --for=condition=complete job/migrate-volume -n openebs --timeout=550s
kubectl logs --tail=50 -l  job-name=migrate-volume -n openebs

echo "Scaling up application after migration"

kubectl scale statefulset busybox --replicas=1
sleep 5
kubectl wait --for=condition=Ready pod -l lkey=lvalue --timeout=600s 