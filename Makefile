GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
BINARY_NAME=kubectl-xtop
VERSION := $(shell cat .version)
GIT_SHA := $(shell git describe --always --long --dirty)

.PHONY: list build build-linux clean

ifndef VERSION
$(error VERSION env variable is not set)
endif

list:
	@$(MAKE) -pRrq -f $(lastword $(MAKEFILE_LIST)) : 2>/dev/null | awk -v RS= -F: '/^# File/,/^# Finished Make data base/ {if ($$1 !~ "^[#.]") {print $$1}}' | sort | egrep -v -e '^[^[:alnum:]]' -e '^$@$$' | xargs
build:
	$(GOBUILD) -v -ldflags="-X 'github.com/akomic/kubectl-xtop/cmd.Version=${VERSION}' -X 'github.com/akomic/kubectl-xtop/cmd.Sha=${GIT_SHA}' -X 'github.com/akomic/kubectl-xtop/cmd.FileName=kubectl-xtop'" -o $(BINARY_NAME) 
install:
	mv $(BINARY_NAME) /usr/local/bin/kubectl-xtop
	chmod 755 /usr/local/bin/kubectl-xtop
build-linux:
	GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags="-X 'github.com/akomic/kubectl-xtop/cmd.Version=${VERSION}' -X 'github.com/akomic/kubectl-xtop/cmd.Sha=${GIT_SHA}' -X 'github.com/akomic/kubectl-xtop/cmd.FileName=kubectl-xtop.linux.amd64' -s -w" -o $(BINARY_NAME).linux.amd64
build-darwin:
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -ldflags="-X 'github.com/akomic/kubectl-xtop/cmd.Version=${VERSION}' -X 'github.com/akomic/kubectl-xtop/cmd.Sha=${GIT_SHA}' -X 'github.com/akomic/kubectl-xtop/cmd.FileName=kubectl-xtop.darwin.amd64' -s -w" -o $(BINARY_NAME).darwin.amd64
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -ldflags="-X 'github.com/akomic/kubectl-xtop/cmd.Version=${VERSION}' -X 'github.com/akomic/kubectl-xtop/cmd.Sha=${GIT_SHA}' -X 'github.com/akomic/kubectl-xtop/cmd.FileName=kubectl-xtop.darwin.arm64' -s -w" -o $(BINARY_NAME).darwin.arm64
mod-vendor:
	go mod vendor
github-release:
ifndef GITHUB_TOKEN
	echo "GITHUB_TOKEN env variable is not set"
	exit 1
endif

	gh release create $(VERSION) kubectl-xtop.darwin.amd64 kubectl-xtop.darwin.arm64 kubectl-xtop.linux.amd64 -t $(VERSION) --notes $(VERSION)
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME) $(BINARY_NAME).linux.amd64 $(BINARY_NAME).darwin.amd64 $(BINARY_NAME).darwin.arm64

all: clean build-darwin build-linux

release: all github-release
