.PHONY: build
build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build

upload:
	aws --endpoint=https://storage.yandexcloud.net s3 cp yandex-guest-agent s3://nikolaymatrosov.ru/uploads/yandex-guest-agent
