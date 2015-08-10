GIT_SHA := $(shell git log -1 --pretty=format:"%h")

all: install

clean:
	go clean ./...

doc:
	godoc -http=:6060

install:
	go get github.com/blang/semver
	go get github.com/chop-dbhi/data-models-service/client

test-install: install
	go get golang.org/x/tools/cmd/cover
	go get github.com/cespare/prettybench

dev-install: install test-install

test:
	go test -cover ./...

build:
	go build \
		-ldflags "-X validator.progBuild '$(GIT_SHA)'" \
		-o $(GOPATH)/bin/data-models-validator ./cmd/validator

bench:
	go test -run=none -bench=. ./... | prettybench

fmt:
	go vet ./...
	go fmt ./...

lint:
	golint ./...

.PHONY: test
