PROJECT = portfolio
IMAGE = registry.gitlab.com/moderntoken/$(PROJECT)
IMAGE_TAG ?= latest
COVERAGE_PATH=coverage.out

.PHONY: test

help: ## Show this help.
	@fgrep -h "##" $(MAKEFILE_LIST) | fgrep -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'

install-tools:
	go install github.com/kyleconroy/sqlc/cmd/sqlc@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/vektra/mockery/v2@latest
	go install github.com/swaggo/swag/cmd/swag@latest

linters: ## Get the list of enabled and disabled linters with description
	golangci-lint linters

lint: ## Run linter
	golangci-lint run

test: ## Run go tests
	go test ./... -count=1 -coverprofile=$(COVERAGE_PATH)

cover: ## Watch code coverage in browser
	go tool cover -html=$(COVERAGE_PATH)

build: ## Build docker image
	@if [ -z ${GITLAB_USER} ] || [ -z ${GITLAB_TOKEN} ]; then \
		echo "GITLAB_USER and GITLAB_TOKEN variables are required"; \
		exit 1; \
	fi
	docker build \
		--build-arg VERSION=$(IMAGE_TAG) \
		--build-arg GITLAB_USER=$(GITLAB_USER) \
		--build-arg GITLAB_TOKEN=$(GITLAB_TOKEN) \
		-t $(IMAGE):$(IMAGE_TAG) .

# Infra
infra-up: ## Up docker compose infrastructure
	docker-compose -p $(PROJECT) -f docker-compose.yaml up --detach --remove-orphans
	docker run --net portfolio_default --rm dokku/wait -c rabbitmq:5672 -t 10
	docker exec -it rabbitmq rabbitmqctl import_definitions /definitions.json

infra-down: ## Down docker compose infrastructure
	docker-compose -p $(PROJECT) -f docker-compose.yaml down --remove-orphans

# Generators
gen-sqlc: ## Generate Go methods from SQL queries
	sqlc generate

gen-swagger: ## Generate swagger docs
	swag init -o api/rest/docs --parseDependency

MOCKS_OUT=test/mocks
gen-mocks: ## Generate Go interface mocks for testing
	mockery --dir=pg/repo --name=Querier --filename=querier.go --structname=Querier --output=$(MOCKS_OUT)
	mockery --dir=domain/gateways --name=Manager --filename=gateways_manager.go --structname=GatewaysManager --output=$(MOCKS_OUT)
	mockery --name=Gateway --srcpkg=gitlab.com/moderntoken/gateways/core --filename=gateway.go --structname=Gateway --output=$(MOCKS_OUT)
	mockery --name=Account --srcpkg=gitlab.com/moderntoken/gateways/core --filename=account.go --structname=Account --output=$(MOCKS_OUT)
	mockery --name=Instrument --srcpkg=gitlab.com/moderntoken/gateways/core --filename=instrument.go --structname=Instrument --output=$(MOCKS_OUT)
	mockery --name=UniversalClient --srcpkg=github.com/go-redis/redis/v9 --filename=redis_client.go --structname=RedisClient --output=$(MOCKS_OUT)
