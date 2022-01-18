.PHONY: test
test:
	go test ./... -coverprofile=coverage.out -race

.PHONY: lint
lint:
	golangci-lint run

.PHONY: catalog
catalog:
	go run github.com/argoproj-labs/argocd-notifications/hack/gen catalog
	go run github.com/argoproj-labs/argocd-notifications/hack/gen docs

.PHONY: tools
tools:
	go install github.com/golang/mock/mockgen@v1.5.0

.PHONY: generate
generate: tools
	go generate ./...

.PHONY: trivy
trivy:
	@trivy fs --clear-cache
	@trivy fs .
