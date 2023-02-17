.PHONY: build
build:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build

NEBIUS_LDFLAGS="-X 'main.version=dev-nebius' \
			    -X 'marketplace-yaga/windows/internal/updater.VersionRemoteEndpoint=https://storage.il.nebius.cloud' \
			    -X 'marketplace-yaga/windows/internal/updater.GuestAgentBucket=yc-guestagent'"
build-nebius:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
		-ldflags=$(NEBIUS_LDFLAGS) \
		-o nebius-guest-agent-updater.exe
