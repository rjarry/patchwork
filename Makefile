MANAGE_PY := docker-compose run --rm web python manage.py
GO ?= go
PYTHON ?= python3
V ?= 0

default: pw

src = $(shell git ls-files '*.go')

pw: $(src)
	$(GO) build -trimpath -o pw ./cmd/pw

.state/docker-build: docker-compose.yml tools/docker/Dockerfile requirements-dev.txt
	docker-compose build
	mkdir -p .state
	touch .state/docker-build

build:
	docker-compose build
	mkdir -p .state
	touch .state/docker-build

serve: .state/docker-build
	docker-compose up

tests: .state/docker-build
	docker-compose run -e TOXENV=${TOXENV} --rm web tox
	$(GO) test $(if $(filter 1,$(V)),-v,) ./...

manage: .state/docker-build
	$(MANAGE_PY) $(CMD)

dbshell: .state/docker-build
	$(MANAGE_PY) dbshell

shell: .state/docker-build
	$(MANAGE_PY) shell

migrate: .state/docker-build
	$(MANAGE_PY) migrate

makemigrations: .state/docker-build
	$(MANAGE_PY) makemigrations

dbbackup: .state/docker-build
	$(MANAGE_PY) dbbackup

dbrestore: .state/docker-build
	$(MANAGE_PY) dbrestore

.PHONY: lint
lint:
	@echo '[gofumpt]'
	@! gofumpt -d . | grep ^diff || { \
		echo 'error: above files need reformatting'; \
		exit 1; \
	}
	@echo '[govet]'
	@go vet ./...

.PHONY: format
format:
	gofumpt -w .

.PHONY: default build serve tests dbshell shell manage migrate makemigrations dbbackup dbrestore
