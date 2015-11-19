GIT_SHA := $(shell git log -1 --pretty=format:"%h")

install:
	go get github.com/blang/semver
	go get github.com/chop-dbhi/data-models-service/client
	go get github.com/olekukonko/tablewriter

test-install: install
	go get golang.org/x/tools/cmd/cover

build-install:
	go get github.com/mitchellh/gox

test:
	go test -cover ./...

bench:
	go test -run=none -bench=. ./...

build:
	go build \
		-ldflags "-X validator.progBuild='$(GIT_SHA)'" \
		-o $(GOPATH)/bin/data-models-validator ./cmd/validator

# Build and tag binaries for each OS and architecture.
dist-build:
	mkdir -p dist

	gox -output="dist/{{.OS}}-{{.Arch}}/data-models-validator" \
		-ldflags "-X validator.progBuild='$(GIT_SHA)'" \
		-os="linux windows darwin" \
		-arch="amd64" \
		./cmd/validator > /dev/null

dist-zip:
	cd dist && zip linux-amd64.zip linux-amd64/*
	cd dist && zip windows-amd64.zip windows-amd64/*
	cd dist && zip darwin-amd64.zip darwin-amd64/*

dist: dist-build dist-zip


.PHONY: test build dist-build dist
