NAME := atlona-event-forwarder
OWNER := byuoitav
PKG := github.com/${OWNER}/${NAME}
DOCKER_URL := docker.pkg.github.com

# version:
# use the git tag, if this commit
# doesn't have a tag, use the git hash
VERSION := $(shell git rev-parse HEAD)
ifneq ($(shell git describe --exact-match --tags HEAD 2> /dev/null),)
	VERSION = $(shell git describe --exact-match --tags HEAD)
endif

# go stuff
PKG_LIST := $(shell go list ${PKG}/...)

.PHONY: all deps build test test-cov clean

all: clean build

test:
	@go test -v ${PKG_LIST} && pwd

test-cov:
	@go test -coverprofile=coverage.txt -covermode=atomic ${PKG_LIST}

lint:
	@golangci-lint run --tests=false

deps:
	@echo Downloading dependencies...
	@go mod download

build: deps
	@mkdir -p dist
	@echo Building server...
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o ./dist/${NAME}-linux-amd64 ${PKG}

	@echo Build output is located in ./dist/.

docker: clean build
	@echo Building container ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}:${VERSION}
	@docker build -f dockerfile -t ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}:${VERSION} dist

deploy: docker
	@echo Logging into Github Package Registry
	@docker login ${DOCKER_URL} -u ${DOCKER_USERNAME} -p ${DOCKER_PASSWORD}

	@echo Pushing container ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}:${VERSION}
	@docker push ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}:${VERSION}

clean:
	@go clean
	@rm -rf dist/
