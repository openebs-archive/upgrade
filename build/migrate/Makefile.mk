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

# Specify the name for the binaries
MIGRATE=migrate

# build migrate binary
.PHONY: migrate
migrate:
	@echo "----------------------------"
	@echo "--> ${MIGRATE}              "
	@echo "----------------------------"
	@# PNAME is the sub-folder in ./bin where binary will be placed. 
	@# CTLNAME indicates the folder/pkg under cmd that needs to be built. 
	@# The output binary will be: ./bin/${PNAME}/<os-arch>/${CTLNAME}
	@# A copy of the binary will also be placed under: ./bin/${PNAME}/${CTLNAME}
	@PNAME=${MIGRATE} CTLNAME=${MIGRATE} CGO_ENABLED=0 sh -c "'$(PWD)/build/build.sh'"

# build migrate image
.PHONY: migrate-image.amd64
migrate-image.amd64: migrate
	@echo "-----------------------------------------------"
	@echo "--> ${MIGRATE} image                           "
	@echo "${IMAGE_ORG}/${MIGRATE_REPO_NAME_AMD64}:${IMAGE_TAG}"
	@echo "-----------------------------------------------"
	@cp bin/${MIGRATE}/${MIGRATE} build/${MIGRATE}/
	@cd build/${MIGRATE} && \
	 sudo docker build -t "${IMAGE_ORG}/${MIGRATE_REPO_NAME_AMD64}:${IMAGE_TAG}" ${DBUILD_ARGS} .
	@rm build/${MIGRATE}/${MIGRATE}

# build migrate image
.PHONY: migrate-image.arm64
migrate-image.arm64: migrate
	@echo "-----------------------------------------------"
	@echo "--> ${MIGRATE} image                           "
	@echo "${IMAGE_ORG}/${MIGRATE_REPO_NAME_ARM64}:${IMAGE_TAG}"
	@echo "-----------------------------------------------"
	@cp bin/${MIGRATE}/${MIGRATE} build/${MIGRATE}/
	@cd build/${MIGRATE} && \
	 sudo docker build -t "${IMAGE_ORG}/${MIGRATE_REPO_NAME_ARM64}:${IMAGE_TAG}" ${DBUILD_ARGS} .
	@rm build/${MIGRATE}/${MIGRATE}

# cleanup migrate build
.PHONY: cleanup-migrate
cleanup-migrate: 
	rm -rf ${GOPATH}/bin/${MIGRATE}
