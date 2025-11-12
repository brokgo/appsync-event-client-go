.PHONY: build
build:
	cd lambda && go build -o build/bootstrap
	zip infrastructure/build/lambda.zip lambda/build/bootstrap

.PHONY: deploy
deploy: build
	cd infrastructure && terraform apply

.PHONY: test
test:
	go test -timeout 10s -coverprofile cover.out -coverpkg=github.com/brokgo/appsync-event-client-go/appsync
	go tool cover -func cover.out | fgrep total | awk '{print substr($$3, 1, length($$3)-1)}' | awk '{if($$1<50)exit 1}'
