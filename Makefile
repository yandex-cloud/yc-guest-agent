.PHONY: checksum-guestagent checksum-guestagent-updater build-guestagent build-guestagent-updater sync-to-github

VERSION?=0.0.0

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

GO_BUILD_PARAMS    = GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-buildid=none -X main.version=${VERSION}" -trimpath
GO_CHECKSUM_PARAMS = GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-buildid=none -X main.version=${VERSION}" -trimpath

checksum-guestagent: go-clean
	@echo "+ $@"
	${GO_CHECKSUM_PARAMS} -o yandex-guest-agent.exe.checksum windows/cmd/yandex-guest-agent/yandex-guest-agent.go
	sha256sum yandex-guest-agent.exe.checksum
	rm yandex-guest-agent.exe.checksum

checksum-guestagent-updater: go-clean
	@echo "+ $@"
	${GO_CHECKSUM_PARAMS} -o yandex-guest-agent-updater.exe.checksum windows/cmd/yandex-guest-agent-updater/yandex-guest-agent-updater.go
	sha256sum yandex-guest-agent-updater.exe.checksum
	rm yandex-guest-agent-updater.exe.checksum


build-guestagent: go-clean
	@echo "+ $@"
	${GO_BUILD_PARAMS} -o yandex-guest-agent.exe windows/cmd/yandex-guest-agent/yandex-guest-agent.go

build-guestagent-updater: go-clean
	@echo "+ $@"
	${GO_BUILD_PARAMS} -o yandex-guest-agent-updater.exe windows/cmd/yandex-guest-agent-updater/yandex-guest-agent-updater.go

# Docker preparation
image:
	@echo "+ $@"
	@docker build -t ${DOCKER_IMAGE_TAG} -f ${DOCKERFILE} .
