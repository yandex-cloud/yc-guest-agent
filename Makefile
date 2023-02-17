.PHONY: checksum-guestagent checksum-guestagent-updater build-guestagent build-guestagent-updater sync-to-github

VERSION?=0.0.0
ENVIRONMENT?=Yandex

all:

go-clean:
	@echo "+ $@"
	go clean -cache

DOCKER_IMAGE_TAG = yaga-build-env:${VERSION}
DOCKERFILE = Dockerfile

RUN_DOCKER = docker run --rm --name yaga-builder --env VERSION=$(VERSION) ${DOCKER_IMAGE_TAG}  make ${TARGET}

docker-%:
	@echo "+ $@"
	$(eval INNER_TARGET:= $(shell echo $@ | awk '{sub(/docker-/, "")} 1'))
	@echo "+ Run target \`$(INNER_TARGET)\` inside Docker"
	$(RUN_DOCKER) $(INNER_TARGET)

# Default build params (Yandex Cloud)
LDFLAGS = -buildid=none -X main.version=${VERSION}
GO_BUILD_PARAMS    = GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="${LDFLAGS}" -trimpath
GO_CHECKSUM_PARAMS = GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="${LDFLAGS}" -trimpath
GUEST_AGENT_NAME = yandex-guest-agent.exe
GUEST_AGENT_UPDATER_NAME = yandex-guest-agent-updater.exe

# Build params for Nebius
ifeq ($(ENVIRONMENT),Nebius) # Nebius
NEBIUS_LDFLAGS = $(LDFLAGS) \
			-X 'marketplace-yaga/windows/internal/updater.VersionRemoteEndpoint=https://storage.il.nebius.cloud' \
			-X 'marketplace-yaga/windows/internal/updater.GuestAgentBucket=yc-guestagent'
GO_BUILD_PARAMS    = GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(NEBIUS_LDFLAGS)" -trimpath
GO_CHECKSUM_PARAMS = GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(NEBIUS_LDFLAGS)" -trimpath
GUEST_AGENT_NAME = nebius-guest-agent.exe
GUEST_AGENT_UPDATER_NAME = nebius-guest-agent-updater.exe
endif

checksum-guestagent: go-clean
	@echo "+ $@"
	${GO_CHECKSUM_PARAMS} -o ${GUEST_AGENT_NAME}.checksum windows/cmd/yandex-guest-agent/yandex-guest-agent.go
	sha256sum ${GUEST_AGENT_NAME}.checksum
	rm ${GUEST_AGENT_NAME}.checksum

checksum-guestagent-updater: go-clean
	@echo "+ $@"
	${GO_CHECKSUM_PARAMS} -o ${GUEST_AGENT_UPDATER_NAME}.checksum windows/cmd/yandex-guest-agent-updater/yandex-guest-agent-updater.go
	sha256sum ${GUEST_AGENT_UPDATER_NAME}.checksum
	rm ${GUEST_AGENT_UPDATER_NAME}.checksum


build-guestagent: go-clean
	@echo "+ $@"
	${GO_BUILD_PARAMS} -o ${GUEST_AGENT_NAME} windows/cmd/yandex-guest-agent/yandex-guest-agent.go

build-guestagent-updater: go-clean
	@echo "+ $@"
	@echo ${ENVIRONMENT}
	@echo ${NEBIUS_LDFLAGS}
	${GO_BUILD_PARAMS} -o ${GUEST_AGENT_UPDATER_NAME} windows/cmd/yandex-guest-agent-updater/yandex-guest-agent-updater.go

# Docker preparation
image:
	@echo "+ $@"
	@docker build -t ${DOCKER_IMAGE_TAG} -f ${DOCKERFILE} .
