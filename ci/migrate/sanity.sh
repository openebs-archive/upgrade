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