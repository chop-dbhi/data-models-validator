BUILD_VERSION=$(shell git log -1 --pretty=format:"%h (%ci)")
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)


# Build and tag binaries for each OS and architecture.
build:
	mkdir -p dist

	go build \
		-o "dist/$(GOOS)-$(GOARCH)/data-models-validator" \
		-ldflags "-X 'validator.progBuild=$(BUILD_VERSION)' -extldflags '-static'" \
		./cmd/validator

zip-build:
	cd dist && zip data-models-validator-$(GOOS)-$(GOARCH).zip $(GOOS)-$(GOARCH)/*

dist:
	GOOS=darwin GOARCH=amd64 make build zip-build
	GOOS=linux GOARCH=amd64 make build zip-build
	GOOS=windows GOARCH=amd64 make build zip-build

.PHONY: dist
