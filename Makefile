.PHONY: all clean fmt build test

BINARY := strimzi-shutdown

all: build test

clean:
	go clean -x
	go clean -testcache -x
	rm -fv $(BINARY)
	rm -fv $(BINARY)-*

fmt:
	gofmt -w main.go cmd/*.go

build:
	gofmt -w main.go cmd/*.go
	go vet ./...
	go mod tidy
	go build -v -o $(BINARY)

test:
	go test -v ./...
