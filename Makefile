UNIT_TEST_DIRS := $(shell go list ./... | grep -v github.com/brokgo/appsync-event-client-go/e2e)

.PHONY: e2e-test
e2e-test:
	cd e2e && make test

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: lint
lint:
	golangci-lint run

.PHONY: unit-test
unit-test:
	go test $(UNIT_TEST_DIRS) -coverprofile cover.out
	go tool cover -func cover.out | fgrep total | awk '{print substr($$3, 1, length($$3)-1)}' | awk '{if($$1<80)exit 1}'
