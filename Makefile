.PHONY: build
build:
	go build -o build/privateInfoBot_arm64

.PHONY: build-arm64
build-arm64:
	env GOOS=linux GOARCH=arm64 go build -o build/privateInfoBot_arm64
