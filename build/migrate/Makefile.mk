
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
	@echo "${HUB_USER}/${MIGRATE_REPO_NAME_AMD64}:${IMAGE_TAG}"
	@echo "-----------------------------------------------"
	@cp bin/${MIGRATE}/${MIGRATE} build/${MIGRATE}/
	@cd build/${MIGRATE} && \
	 sudo docker build -t "${HUB_USER}/${MIGRATE_REPO_NAME_AMD64}:${IMAGE_TAG}" ${DBUILD_ARGS} .
	@rm build/${MIGRATE}/${MIGRATE}

# build migrate image
.PHONY: migrate-image.arm64
migrate-image.arm64: migrate
	@echo "-----------------------------------------------"
	@echo "--> ${MIGRATE} image                           "
	@echo "${HUB_USER}/${MIGRATE_REPO_NAME_ARM64}:${IMAGE_TAG}"
	@echo "-----------------------------------------------"
	@cp bin/${MIGRATE}/${MIGRATE} build/${MIGRATE}/
	@cd build/${MIGRATE} && \
	 sudo docker build -t "${HUB_USER}/${MIGRATE_REPO_NAME_ARM64}:${IMAGE_TAG}" ${DBUILD_ARGS} .
	@rm build/${MIGRATE}/${MIGRATE}

# cleanup migrate build
.PHONY: cleanup-migrate
cleanup-migrate: 
	rm -rf ${GOPATH}/bin/${MIGRATE}
