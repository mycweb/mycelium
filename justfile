
all: tidy test

generate:
	go generate ./...

tidy: generate
	go mod tidy
	go fmt ./...
	go vet ./...

test: generate
	mkdir -p ./build/out
	go test -coverprofile=build/out/go-cover ./...

coverage:
	go tool cover -html ./build/out/go-cover
