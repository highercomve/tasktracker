BINARY_NAME=tasktracker
ENTRY_POINT=./cmd/tasktracker
VERSION=$(shell git describe --tags --always --dirty="-dev" --abbrev=7)
LINUX_AMD64_LIBS = /usr/lib /usr/lib64 /usr/lib/x86_64-linux-gnu
LINUX_ARM64_LIBS = /usr/lib /usr/lib64 /usr/lib/aarch64-linux-gnu
LINUX_ARM_LIBS = /usr/lib /usr/lib64 /usr/lib/arm-linux-gnueabihf
LINUX_AMD64_LDFLAGS = $(foreach dir,$(wildcard $(LINUX_AMD64_LIBS)),-L$(dir))
LINUX_ARM64_LDFLAGS = $(foreach dir,$(wildcard $(LINUX_ARM64_LIBS)),-L$(dir))
LINUX_ARM_LDFLAGS = $(foreach dir,$(wildcard $(LINUX_ARM_LIBS)),-L$(dir))

EXE_EXT_windows = .exe
EXE_EXT_linux =

LDFLAGS_windows = -H=windowsgui

ZIG_CC_FLAGS_windows = -Wdeprecated-non-prototype -Wl,--subsystem,windows
ZIG_CC_FLAGS_linux-amd64 = -isystem /usr/include $(LINUX_AMD64_LDFLAGS) # Native build
ZIG_CC_FLAGS_linux-arm64 = -isystem /usr/include $(LINUX_ARM64_LDFLAGS)
ZIG_CC_FLAGS_linux-arm = -isystem /usr/include $(LINUX_ARM_LDFLAGS)

ZIG_TARGET_linux-amd64 = x86_64-linux-gnu
ZIG_TARGET_linux-arm64 = aarch64-linux-gnu
ZIG_TARGET_linux-arm = arm-linux-gnueabihf
ZIG_TARGET_windows-amd64 = x86_64-windows-gnu
ZIG_TARGET_windows-arm64 = aarch64-windows-gnu

.PHONY: all build run clean test deps build-linux build-windows debian-deps arch-deps release

all: build-linux-amd64 build-windows-amd64

build:
	go build -ldflags="-X 'github.com/highercomve/tasktracker/internal/version.Version=$(VERSION)'" -o $(BINARY_NAME) $(ENTRY_POINT)

run:
	go run $(ENTRY_POINT)

clean:
	go clean
	rm -f tasktracker*
	rm -rf dist
	rm -rf fyne-cross

test:
	go test -v ./...

deps:
	go mod tidy

# Generic build rule for cross-compilation
# Usage: make build-OS-ARCH (e.g., make build-linux-amd64)
build-%: deps
	$(eval GOOS := $(word 1,$(subst -, ,$*)))
	$(eval GOARCH := $(word 2,$(subst -, ,$*)))
	@echo "Building for $(GOOS) ($(GOARCH))"
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=1 \
	CC="zig cc -target $(ZIG_TARGET_$*) $(ZIG_CC_FLAGS_$*)" \
	CXX="zig c++ -target $(ZIG_TARGET_$*) $(ZIG_CC_FLAGS_$*)" \
	go build -ldflags="-X 'github.com/highercomve/tasktracker/internal/version.Version=$(VERSION)' $(LDFLAGS_$(GOOS))" -o dist/$*/$(BINARY_NAME)$(EXE_EXT_$(GOOS)) $(ENTRY_POINT)

# Alias for backward compatibility
build-linux: build-linux-amd64
build-windows: build-windows-amd64

release: release-linux-amd64 release-linux-arm64 release-windows-amd64

release-linux-%: package-linux-%
	cd dist/linux-$* && tar -cJf ../../tasktracker-linux-$*.tar.xz tasktracker

release-windows-%: package-windows-%
	cd dist/windows-$* && zip ../../tasktracker-windows-$*.zip tasktracker.exe

debian-deps:
	@echo "Installing Debian/Ubuntu dependencies for Fyne build..."
	sudo apt-get update && sudo apt-get install -y \
	build-essential libgl1-mesa-dev xorg-dev libxcursor-dev libxrandr-dev \
	libxi-dev libxkbcommon-dev gcc linux-libc-dev libxxf86vm-dev


arch-deps:
	@echo "Installing Arch Linux dependencies for Fyne build..."
	sudo pacman -Syu --needed base-devel mesa libxkbcommon \
	aarch64-linux-gnu-gcc aarch64-linux-gnu-libx11 aarch64-linux-gnu-libxcursor aarch64-linux-gnu-libxrandr aarch64-linux-gnu-libxinerama aarch64-linux-gnu-libxi aarch64-linux-gnu-libxkbcommon \
	arm-linux-gnueabihf-gcc arm-linux-gnueabihf-libx11 arm-linux-gnueabihf-libxcursor arm-linux-gnueabihf-libxrandr arm-linux-gnueabihf-libxinerama arm-linux-gnueabihf-libxi arm-linux-gnueabihf-libxkbcommon

package-%: go.mod $(wildcard cmd/*.go)
	$(eval GOOS := $(word 1,$(subst -, ,$*)))
	$(eval GOARCH := $(word 2,$(subst -, ,$*)))
	@echo "Building CLI for $(GOOS) ($(GOARCH))"
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=1 \
	fyne-cross $(GOOS) -arch=$(GOARCH) \
		-name $(BINARY_NAME)$(EXE_EXT_$(GOOS)) \
		-icon Icon.png \
		--app-id com.highercomve.tasktracker \
		-ldflags="github.com/highercomve/tasktracker/internal/version.Version=$(VERSION)" \
		$(ENTRY_POINT)
	@mkdir -p dist/$(GOOS)-$(GOARCH)
	cp fyne-cross/bin/$(GOOS)-$(GOARCH)/$(BINARY_NAME)$(EXE_EXT_$(GOOS)) dist/$(GOOS)-$(GOARCH)/$(BINARY_NAME)$(EXE_EXT_$(GOOS))
