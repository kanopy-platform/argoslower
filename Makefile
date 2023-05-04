GO_MODULE := $(shell git config --get remote.origin.url | grep -o 'github\.com[:/][^.]*' | tr ':' '/')
CMD_NAME := $(shell basename ${GO_MODULE})
DEFAULT_APP_PORT ?= 8080
GIT_COMMIT := $(shell git rev-parse HEAD)
ENVTEST_K8S_VERSION = 1.21.4 # matches latest binary version available

RUN ?= .*
PKG ?= ./...


.PHONY: test
test: tidy ## Run tests in local environment
	golangci-lint run --timeout=5m $(PKG)
	go test -cover -short -run=$(RUN) $(PKG)

.PHONY: integration
integration: tidy envtest ## Run integration tests with envtest
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --arch amd64 -p path)" go test -cover -run=$(RUN) $(PKG)

.PHONY: dev
dev: tidy
	mkdir -p bin
	go build -o bin/${CMD_NAME} ./cmd/

.PHONY:
tidy:
	go mod tidy
	go mod verify


.PHONY: docker-test
docker-test: ## Run tests using local development docker image
	@docker run -v $(pwd):/go/src/app/ --workdir=/go/src/app golang:1.20  go test -cover --coverprofile=coverage ./...

.PHONY: docker-snyk
docker-snyk: ## Run local snyk scan, SNYK_TOKEN environment variable must be set
	@docker run --rm -e SNYK_TOKEN -w /go/src/$(GO_MODULE) -v $(shell pwd):/go/src/$(GO_MODULE):delegated snyk/snyk:golang

.PHONY: docker
docker:
	@docker build --build-arg GIT_COMMIT=${GIT_COMMIT} -t $(CMD_NAME):latest .

.PHONY: docker-run
docker-run: docker ## Build and run the application in a local docker container
	@docker run -p ${DEFAULT_APP_PORT}:${DEFAULT_APP_PORT} $(CMD_NAME):latest

.PHONY: help
help:
