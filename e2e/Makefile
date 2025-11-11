.PHONY: build
build:
	cd lambda && go build -o build/bootstrap
	zip infrastructure/build/lambda.zip lambda/build/bootstrap

.PHONY: deploy
deploy: build
	cd infrastructure && terraform apply

.PHONY: test
test:
	go test -timeout 10s
