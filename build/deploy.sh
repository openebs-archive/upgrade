#!/bin/bash
# Copyright 2020 The OpenEBS Authors.
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

set -e

# Determine the arch/os we're building for
ARCH=$(uname -m)

if [ "${ARCH}" = "x86_64" ]; then
  UPGRADE_IMG="${IMAGE_ORG}/upgrade-amd64"
  MIGRATE_IMG="${IMAGE_ORG}/migrate-amd64"
elif [ "${ARCH}" = "aarch64" ]; then
  UPGRADE_IMG="${IMAGE_ORG}/upgrade-arm64"
  MIGRATE_IMG="${IMAGE_ORG}/migrate-arm64"
fi

# tag and push all the images
DIMAGE="${UPGRADE_IMG}" ./build/push
DIMAGE="${MIGRATE_IMG}" ./build/push