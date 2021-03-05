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

export DBUILD_ARGS=--build-arg DBUILD_DATE=${DBUILD_DATE} --build-arg DBUILD_REPO_URL=${DBUILD_REPO_URL} --build-arg DBUILD_SITE_URL=${DBUILD_SITE_URL} --build-arg RELEASE_TAG=${RELEASE_TAG} --build-arg BRANCH=${BRANCH}

# Specify the name for the binaries
UPGRADE=upgrade
MIGRATE=migrate

# If there are any external tools need to be used, they can be added by defining a EXTERNAL_TOOLS variable 
# Bootstrap the build by downloading additional tools
.PHONY: bootstrap
bootstrap:
	@for tool in  $(EXTERNAL_TOOLS) ; do \
		echo "Installing $$tool" ; \
		go get -u $$tool; \
	done

.PHONY: clean-migrate
clean-migrate:
	@echo '--> Cleaning migrate directory...'
	rm -rf bin/${MIGRATE}
	rm -rf ${GOPATH}/bin/${MIGRATE}
	@echo '--> Done cleaning.'
	@echo

.PHONY: clean-upgrade
clean-upgrade:
	@echo '--> Cleaning upgrade directory...'
	rm -rf bin/${UPGRADE}
	rm -rf ${GOPATH}/bin/${UPGRADE}
	@echo '--> Done cleaning.'
	@echo

# deps ensures fresh go.mod and go.sum.
.PHONY: deps
deps:
	@echo "--> Tidying up submodules"
	@go mod tidy
	@echo "--> Verifying submodules"
	@go mod verify

.PHONY: test
test:
	go test ./...

# Specify the name of the docker repo for amd64
UPGRADE_REPO_NAME_AMD64="upgrade-amd64"
MIGRATE_REPO_NAME_AMD64="migrate-amd64"

ifeq (${IMAGE_TAG}, )
  IMAGE_TAG = ci
  export IMAGE_TAG
endif

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

# build upgrade image
.PHONY: upgrade-image.amd64
upgrade-image.amd64: upgrade
	@echo "-----------------------------------------------"
	@echo "--> ${UPGRADE} image                           "
	@echo "${IMAGE_ORG}/${UPGRADE_REPO_NAME}:${IMAGE_TAG}"
	@echo "-----------------------------------------------"
	@cp bin/${UPGRADE}/${UPGRADE} build/${UPGRADE}
	@cd build/${UPGRADE} && \
	 sudo docker build -t "${IMAGE_ORG}/${UPGRADE_REPO_NAME_AMD64}:${IMAGE_TAG}" ${DBUILD_ARGS} .
	@rm build/${UPGRADE}/${UPGRADE}

# cleanup upgrade build
.PHONY: cleanup-upgrade
cleanup-upgrade: 
	rm -rf ${GOPATH}/bin/${UPGRADE}

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

# cleanup migrate build
.PHONY: cleanup-migrate
cleanup-migrate: 
	rm -rf ${GOPATH}/bin/${MIGRATE}

.PHONY: all.amd64
all.amd64: upgrade-image.amd64 migrate-image.amd64

# Push images
.PHONY: deploy-images
deploy-images:
	@./build/deploy.sh

.PHONY: check_license
check-license:
	@echo ">> checking license header"
	@licRes=$$(for file in $$(find . -type f -regex '.*\.sh\|.*\.go\|.*Docker.*\|.*\Makefile*\|.*\yaml' ! -path './vendor/*' ) ; do \
               awk 'NR<=3' $$file | grep -Eq "(Copyright|generated|GENERATED)" || echo $$file; \
       done); \
       if [ -n "$${licRes}" ]; then \
               echo "license header checking failed:"; echo "$${licRes}"; \
               exit 1; \
       fi

# include the buildx recipes
include Makefile.buildx.mk
