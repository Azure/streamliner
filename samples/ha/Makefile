REGISTRY=acraettestwestus3.azurecr.io
IMAGENAME=github.com/Azure/streamliner/samples/ha-app
VERSION=1.0.12-debug

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: build-container
build-container: ## build container image
	docker build -t $(REGISTRY)/$(IMAGENAME):$(VERSION) -f ./k8s/Dockerfile .

.PHONY: publish-container
publish-container: build-container ## build and push image to container registry
	docker push $(REGISTRY)/$(IMAGENAME):$(VERSION)

.PHONY: deploy
deploy: ## deploy to AKS cluster
	kubectl apply -f k8s/deployment.yaml

.PHONY: deploy-dev
deploy-test: ## deploy in test environment
	kubectl apply -k ./k8s/dev/

.PHONY: undeploy-dev
undeploy: ## deploy to AKS cluster
	kubectl delete -k ./k8s/dev

.PHONY: lint
lint: ## golangci-lint
	$(call print-target)
	golangci-lint run --fix

.PHONY: build
build: ## goreleaser build
build:
	$(call print-target)
	goreleaser build --rm-dist --single-target --snapshot

.PHONY: clean
clean: ## remove files created during build pipeline
	$(call print-target)
	rm -rf *.out
	rm -rf {base_memprofile.out,prev_memprofile}
