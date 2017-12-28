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
	@echo " -> Building $(REPO) v$(TAG)-@$(COMMIT)"
	@go build -tags "netgo static_build" -installsuffix netgo \
		-ldflags "-w -X github.com/$(REPO)/${PACKAGE}.GitCommit=$(COMMIT)
	@echo "Built $$(./$(APP) -v)"

image:
	@docker build --build-arg TAG=$(TAG) -t $(REPO):$(TAG) .
	@echo "Image created: $(REPO):$(TAG)"

test:
	@go test -v -cover -race $(TEST_ARGS)

clean:
	@rm -rf $(APP)
