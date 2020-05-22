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


# IMAGE_ORG can be used to customize the organization 
# under which images should be pushed. 
# By default the organization name is `openebs`. 

ifeq (${IMAGE_ORG}, )
  IMAGE_ORG = openebs
  export IMAGE_ORG
endif

# Specify the date of build
DBUILD_DATE=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Specify the docker arg for repository url
ifeq (${DBUILD_REPO_URL}, )
  DBUILD_REPO_URL="https://github.com/openebs/upgrade"
  export DBUILD_REPO_URL
endif

# Specify the docker arg for website url
ifeq (${DBUILD_SITE_URL}, )
  DBUILD_SITE_URL="https://openebs.io"
  export DBUILD_SITE_URL
endif

# Determine the arch/os
ifeq (${XC_OS}, )
  XC_OS:=$(shell go env GOOS)
endif
export XC_OS

ifeq (${XC_ARCH}, )
  XC_ARCH:=$(shell go env GOARCH)
endif
export XC_ARCH

ARCH:=${XC_OS}_${XC_ARCH}
export ARCH

export DBUILD_ARGS=--build-arg DBUILD_DATE=${DBUILD_DATE} --build-arg DBUILD_REPO_URL=${DBUILD_REPO_URL} --build-arg DBUILD_SITE_URL=${DBUILD_SITE_URL} --build-arg ARCH=${ARCH}

# deps ensures fresh go.mod and go.sum.
.PHONY: deps
deps:
	@go mod tidy
	@go mod verify

.PHONY: test
test:
	go test ./...

# Specify the name for the binaries
UPGRADE=upgrade

# Specify the name of the docker repo for amd64
UPGRADE_REPO_NAME_AMD64="upgrade-amd64"

# Specify the name of the docker repo for arm64
UPGRADE_REPO_NAME_ARM64="upgrade-arm64"

# build upgrade binary
.PHONY: upgrade
upgrade:
	@echo "----------------------------"
	@echo "--> ${UPGRADE}              "
	@echo "----------------------------"
	@# PNAME is the sub-folder in ./bin where binary will be placed. 
	@# CTLNAME indicates the folder/pkg under cmd that needs to be built. 
	@# The output binary will be: ./bin/${PNAME}/<os-arch>/${CTLNAME}
	@# A copy of the binary will also be placed under: ./bin/${PNAME}/${CTLNAME}
	@PNAME=${UPGRADE} CTLNAME=${UPGRADE} CGO_ENABLED=0 sh -c "'$(PWD)/build/build.sh'"

# docker hub username
HUB_USER?=openebs

ifeq (${IMAGE_TAG}, )
  IMAGE_TAG = ci
  export IMAGE_TAG
endif


# build upgrade image
.PHONY: upgrade-image.amd64
upgrade-image.amd64: upgrade
	@echo "-----------------------------------------------"
	@echo "--> ${UPGRADE} image                           "
	@echo "${HUB_USER}/${UPGRADE_REPO_NAME}:${IMAGE_TAG}"
	@echo "-----------------------------------------------"
	@cp bin/${UPGRADE}/${UPGRADE} build/${UPGRADE}
	@cd build/${UPGRADE} && \
	 sudo docker build -t "${HUB_USER}/${UPGRADE_REPO_NAME_AMD64}:${IMAGE_TAG}" ${DBUILD_ARGS} .
	@rm build/${UPGRADE}/${UPGRADE}

.PHONY: upgrade-image.arm64
upgrade-image.arm64: upgrade
	@echo "-----------------------------------------------"
	@echo "--> ${UPGRADE} image                           "
	@echo "${HUB_USER}/${UPGRADE_REPO_NAME_ARM64}:${IMAGE_TAG}"
	@echo "-----------------------------------------------"
	@cp bin/${UPGRADE}/${UPGRADE} build/${UPGRADE}
	@cd build/${UPGRADE} && \
	 sudo docker build -t "${HUB_USER}/${UPGRADE_REPO_NAME_ARM64}:${IMAGE_TAG}" ${DBUILD_ARGS} .
	@rm build/${UPGRADE}/${UPGRADE}




# cleanup upgrade build
.PHONY: cleanup-upgrade
cleanup-upgrade: 
	rm -rf ${GOPATH}/bin/${UPGRADE}


# Push images
.PHONY: deploy-images
deploy-images:
	@./build/deploy.sh
