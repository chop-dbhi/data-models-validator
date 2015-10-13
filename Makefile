GIT_SHA := $(shell git log -1 --pretty=format:"%h")

install:
	go get github.com/blang/semver
	go get github.com/chop-dbhi/data-models-service/client

test-install: install
	go get golang.org/x/tools/cmd/cover
	go get github.com/cespare/prettybench
	go get github.com/mitchellh/gox

test:
	go test -cover ./...

bench:
	go test -run=none -bench=. ./... | prettybench

build:
	go build \
		-ldflags "-X validator.progBuild='$(GIT_SHA)'" \
		-o $(GOPATH)/bin/data-models-validator ./cmd/validator

# Build and tag binaries for each OS and architecture.
dist:
	mkdir -p bin

	gox -output="dist/data-models-validator-{{.OS}}-{{.Arch}}" \
		-ldflags "-X validator.progBuild='$(GIT_SHA)'" \
		-os="linux windows darwin" \
		-arch="amd64" \
		./cmd/validator > /dev/null

.PHONY: test build dist
