BINARY_NAME=tasktracker
ENTRY_POINT=./cmd/tasktracker
VERSION=$(shell git describe --tags --always --dirty="-dev" --abbrev=7)
LINUX_AMD64_LIBS = /usr/lib /usr/lib64 /usr/lib/x86_64-linux-gnu
LINUX_AMD64_LDFLAGS = $(foreach dir,$(wildcard $(LINUX_AMD64_LIBS)),-L$(dir))

.PHONY: all build run clean test deps build-linux build-windows build-linux-arm64 build-windows-arm64 build-windows-all

all: build-linux build-windows

build:
	go build -ldflags="-X 'github.com/highercomve/tasktracker/internal/version.Version=$(VERSION)'" -o $(BINARY_NAME) $(ENTRY_POINT)

run:
	go run $(ENTRY_POINT)

clean:
	go clean
	rm -rf dist

test:
	go test -v ./...

deps:
	go mod tidy

build-linux: deps
	@echo "Building for Linux (amd64)"
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 CC="zig cc -target x86_64-linux-gnu -isystem /usr/include $(LINUX_AMD64_LDFLAGS)" CXX="zig c++ -target x86_64-linux-gnu -isystem /usr/include $(LINUX_AMD64_LDFLAGS)" go build -ldflags="-X 'github.com/highercomve/tasktracker/internal/version.Version=$(VERSION)'" -o dist/linux-amd64/$(BINARY_NAME) $(ENTRY_POINT)

build-windows: deps
	@echo "Building for Windows (amd64)"
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC="zig cc -target x86_64-windows-gnu -Wdeprecated-non-prototype -Wl,--subsystem,windows" CXX="zig c++ -target x86_64-windows-gnu -Wdeprecated-non-prototype -Wl,--subsystem,windows" go build -ldflags="-X 'github.com/highercomve/tasktracker/internal/version.Version=$(VERSION)'" -o dist/windows-amd64/$(BINARY_NAME).exe $(ENTRY_POINT)

build-windows-arm64: deps
	@echo "Building for Windows (arm64)"
	GOOS=windows GOARCH=arm64 CGO_ENABLED=1 CC="zig cc -target aarch64-windows-gnu -Wdeprecated-non-prototype -Wl,--subsystem,windows" CXX="zig c++ -target aarch64-windows-gnu -Wdeprecated-non-prototype -Wl,--subsystem,windows" go build -ldflags="-X 'github.com/highercomve/tasktracker/internal/version.Version=$(VERSION)'" -o dist/windows-arm64/$(BINARY_NAME).exe $(ENTRY_POINT)
