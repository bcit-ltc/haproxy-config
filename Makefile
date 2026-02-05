.PHONY: help build test docker-build docker-push deploy install uninstall run manifests fmt vet lint helm-lint helm-template helm-install helm-upgrade helm-uninstall

# Image URL to use all building/pushing image targets
IMG ?= haproxy-operator:latest
REGISTRY ?= docker.io/yourorg

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

run: ## Run operator locally against configured Kubernetes cluster
	go run ./main.go \
		--namespace=default \
		--configmap-key=config.yaml

test: fmt vet ## Run tests
	go test ./... -coverprofile cover.out

fmt: ## Run go fmt against code
	go fmt ./...

vet: ## Run go vet against code
	go vet ./...

lint: ## Run golangci-lint
	golangci-lint run

##@ Build

build: fmt vet ## Build manager binary
	go build -o bin/manager main.go

docker-build: ## Build docker image
	docker build -t ${IMG} .

docker-push: ## Push docker image
	docker push ${IMG}

docker-build-push: docker-build docker-push ## Build and push docker image

##@ Deployment

install: ## Install operator using kubectl (deprecated, use helm-install)
	kubectl apply -f config/deployment.yaml

uninstall: ## Uninstall operator using kubectl (deprecated, use helm-uninstall)
	kubectl delete -f config/deployment.yaml

helm-lint: ## Lint Helm chart
	helm lint charts/haproxy-operator

helm-template: ## Render Helm templates
	helm template haproxy-operator charts/haproxy-operator \
		--namespace haproxy-operator-system

helm-install: ## Install operator using Helm
	helm install haproxy-operator charts/haproxy-operator \
		--namespace haproxy-operator-system \
		--create-namespace

helm-upgrade: ## Upgrade operator using Helm
	helm upgrade haproxy-operator charts/haproxy-operator \
		--namespace haproxy-operator-system

helm-uninstall: ## Uninstall operator using Helm
	helm uninstall haproxy-operator \
		--namespace haproxy-operator-system

helm-package: ## Package Helm chart
	helm package charts/haproxy-operator

deploy: docker-build-push helm-upgrade ## Build, push, and deploy operator

deploy-kubectl: docker-build-push install ## Build, push, and deploy operator (kubectl)

##@ Samples

install-samples: ## Install sample ConfigMap and Secret
	kubectl apply -f config/samples/configmap.yaml

uninstall-samples: ## Uninstall sample ConfigMap and Secret
	kubectl delete -f config/samples/configmap.yaml

##@ Utilities

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f cover.out

logs: ## View operator logs
	kubectl logs -n haproxy-operator-system -l app.kubernetes.io/name=haproxy-operator -f

status: ## Check operator status
	kubectl get deployment -n haproxy-operator-system
	kubectl get pods -n haproxy-operator-system

describe: ## Describe operator deployment
	kubectl describe deployment haproxy-operator -n haproxy-operator-system

restart: ## Restart operator
	kubectl rollout restart deployment haproxy-operator -n haproxy-operator-system

##@ Testing

test-configmap: ## Check test ConfigMap status
	kubectl get configmap haproxy-config -o yaml | grep haproxy.operator || echo "No ConfigMap found"

test-connection: ## Test HAProxy API connectivity from cluster
	kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
		curl -v -u admin:password http://192.168.2.2:5555/v2/info

##@ Go Dependencies

mod-download: ## Download go modules
	go mod download

mod-tidy: ## Tidy go modules
	go mod tidy

mod-verify: ## Verify go modules
	go mod verify

mod-update: ## Update go modules
	go get -u ./...
	go mod tidy

##@ Local Development

dev-setup: ## Setup development environment
	go mod download
	@echo "Development environment ready"

dev-run: ## Run operator in development mode
	go run ./main.go \
		--namespace=default \
		--configmap-key=config.yaml \
		--zap-devel=true \
		--zap-log-level=debug

##@ Release

tag: ## Create git tag (usage: make tag VERSION=v1.0.0)
	@if [ -z "$(VERSION)" ]; then echo "VERSION is required"; exit 1; fi
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)

release: ## Build and push release image (usage: make release VERSION=v1.0.0)
	@if [ -z "$(VERSION)" ]; then echo "VERSION is required"; exit 1; fi
	docker build -t $(REGISTRY)/haproxy-operator:$(VERSION) .
	docker tag $(REGISTRY)/haproxy-operator:$(VERSION) $(REGISTRY)/haproxy-operator:latest
	docker push $(REGISTRY)/haproxy-operator:$(VERSION)
	docker push $(REGISTRY)/haproxy-operator:latest
