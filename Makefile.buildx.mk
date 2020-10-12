# Copyright 2018-2020 The OpenEBS Authors. All rights reserved.
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

# ==============================================================================
# Build Options

export DBUILD_ARGS=--build-arg DBUILD_DATE=${DBUILD_DATE} --build-arg DBUILD_REPO_URL=${DBUILD_REPO_URL} --build-arg DBUILD_SITE_URL=${DBUILD_SITE_URL}

ifeq (${TAG}, )
	export TAG=ci
endif


# Build upgrade & migrate docker image with buildx
# Experimental docker feature to build cross platform multi-architecture docker images
# https://docs.docker.com/buildx/working-with-buildx/

# default list of platforms for which multiarch image is built
ifeq (${PLATFORMS}, )
	export PLATFORMS="linux/amd64,linux/arm64,linux/arm/v7,linux/ppc64le"
endif

# if IMG_RESULT is unspecified, by default the image will be pushed to registry
ifeq (${IMG_RESULT}, load)
	export PUSH_ARG="--load"
	# if load is specified, image will be built only for the build machine architecture.
	export PLATFORMS="local"
else ifeq (${IMG_RESULT}, cache)
	# if cache is specified, image will only be available in the build cache, it won't be pushed or loaded
	# therefore no PUSH_ARG will be specified
else
	export PUSH_ARG="--push"
endif

# Name of the multiarch image for upgrade job
DOCKERX_IMAGE_UPGRADE:=${IMAGE_ORG}/upgrade:${TAG}

# Name of the multiarch image for migrate job
DOCKERX_IMAGE_MIGRATE:=${IMAGE_ORG}/migrate:${TAG}

.PHONY: docker.buildx
docker.buildx:
	export DOCKER_CLI_EXPERIMENTAL=enabled
	@if ! docker buildx ls | grep -q container-builder; then\
		docker buildx create --platform ${PLATFORMS} --name container-builder --use;\
	fi
	@docker buildx build --platform ${PLATFORMS} \
		-t "$(DOCKERX_IMAGE_NAME)" ${DBUILD_ARGS} -f $(PWD)/build/$(COMPONENT)/$(COMPONENT).Dockerfile \
		. ${PUSH_ARG}
	@echo "--> Build docker image: $(DOCKERX_IMAGE_NAME)"
	@echo

.PHONY: buildx.upgrade
buildx.upgrade: bootstrap clean-upgrade
	@echo '--> Building upgrade binary...'
	@pwd
	@PNAME=${UPGRADE} CTLNAME=${UPGRADE} BUILDX=true sh -c "'$(PWD)/build/build.sh'"
	@echo '--> Built binary.'
	@echo

.PHONY: docker.buildx.upgrade
docker.buildx.upgrade: DOCKERX_IMAGE_NAME=$(DOCKERX_IMAGE_UPGRADE)
docker.buildx.upgrade: COMPONENT=$(UPGRADE)
docker.buildx.upgrade: docker.buildx

.PHONY: buildx.migrate
buildx.migrate: bootstrap clean-migrate
	@echo '--> Building migrate binary...'
	@pwd
	@PNAME=${MIGRATE} CTLNAME=${MIGRATE} BUILDX=true sh -c "'$(PWD)/build/build.sh'"
	@echo '--> Built binary.'
	@echo

.PHONY: docker.buildx.migrate
docker.buildx.migrate: DOCKERX_IMAGE_NAME=$(DOCKERX_IMAGE_MIGRATE)
docker.buildx.migrate: COMPONENT=$(MIGRATE)
docker.buildx.migrate: docker.buildx

.PHONY: buildx.push.upgrade
buildx.push.upgrade:
	BUILDX=true DIMAGE=${IMAGE_ORG}/upgrade ./build/buildxpush.sh

.PHONY: buildx.push.migrate
buildx.push.migrate:
	BUILDX=true DIMAGE=${IMAGE_ORG}/migrate ./build/buildxpush.sh
