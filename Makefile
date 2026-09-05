.PHONY: build build-linux build-linux-arm64 build-all run test clean docker-build docker-run

BINARY_NAME=searxgo

build:
	go build -ldflags="-s -w" -o $(BINARY_NAME) ./cmd/server

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BINARY_NAME)-linux-amd64 ./cmd/server

build-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $(BINARY_NAME)-linux-arm64 ./cmd/server

build-all: build build-linux build-linux-arm64

run: build
	./$(BINARY_NAME) -port 8184

test:
	go test -v ./...

clean:
	go clean
	rm -f $(BINARY_NAME) $(BINARY_NAME).exe $(BINARY_NAME)-linux*

docker-build:
	docker build -t searxgo:latest .

docker-run:
	docker run -p 8184:8184 --name searxgo --rm searxgo:latest
