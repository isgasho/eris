.PHONY: dev build image test deps clean

CGO_ENABLED=0
COMMIT=`git rev-parse --short HEAD`
APP=eris
PACKAGE=irc
REPO?=prologic/$(APP)
TAG?=latest

all: dev

dev: build
	@./$(APP) run

deps:
	@go get ./...

build: clean deps
	@echo "github.com/$(REPO)/${PACKAGE}.GitCommit=$(COMMIT)"
	@echo " -> Building $(REPO) $(TAG)@$(COMMIT)"
	@go build -tags "netgo static_build" -installsuffix netgo \
		-ldflags "-w -X github.com/$(REPO)/${PACKAGE}.GitCommit=$(COMMIT)"
	@echo "Built $$(./$(APP) -v)"

image:
	@docker build --build-arg TAG=$(TAG) -t $(REPO):$(TAG) .
	@echo "Image created: $(REPO):$(TAG)"

profile:
	@go test -cpuprofile cpu.prof -memprofile mem.prof -v -bench ./...

bench:
	@go test -v -bench ./...

test:
	@go test -v -cover -coverprofile=coverage.txt -covermode=atomic -coverpkg=./... -race ./...

clean:
	@git clean -f -d -X
