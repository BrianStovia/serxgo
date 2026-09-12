.PHONY: build build-linux build-linux-arm64 build-darwin build-darwin-arm64 build-windows build-all run test clean docker-build docker-run

DIST_DIR=dist
BINARY_NAME=searxgo

build:
	mkdir -p $(DIST_DIR)
	go build -ldflags="-s -w" -o $(DIST_DIR)/$(BINARY_NAME) ./cmd/server

build-linux:
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/server

build-linux-arm64:
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/server

build-darwin:
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/server

build-darwin-arm64:
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/server

build-windows:
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/server
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(DIST_DIR)/$(BINARY_NAME).exe ./cmd/server

build-all: build-linux build-linux-arm64 build-darwin build-darwin-arm64 build-windows

run: build
	./$(DIST_DIR)/$(BINARY_NAME) -port 8184

test:
	go test -v ./...

clean:
	go clean
	rm -rf $(DIST_DIR)

docker-build:
	docker build -t searxgo:latest .

docker-run:
	docker run -p 8184:8184 --name searxgo --rm searxgo:latest
