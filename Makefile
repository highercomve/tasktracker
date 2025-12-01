BINARY_NAME=task-tracker
ENTRY_POINT=./cmd/mytracker

.PHONY: all build run clean test deps deps-cross build-linux build-windows dist

all: build

build:
	go build -o $(BINARY_NAME) $(ENTRY_POINT)

run:
	go run $(ENTRY_POINT)

clean:
	go clean
	rm -f $(BINARY_NAME)

test:
	go test -v ./...

deps:
	go mod tidy

deps-cross:
	go install fyne.io/fyne-cross/v2/cmd/fyne-cross@latest

build-linux:
	fyne-cross linux -arch=amd64 -app-id com.highercomve.task-tracker -name $(BINARY_NAME) $(ENTRY_POINT)

build-windows:
	fyne-cross windows -arch=amd64 -app-id com.highercomve.task-tracker -name $(BINARY_NAME) $(ENTRY_POINT)

dist: build-linux build-windows
