MODULE := github.com/domdom82/moo2hd

# Binaries
GAME_BIN    := bin/moo2hd
EXTRACT_BIN := bin/lbxextract
CONVERT_BIN := bin/svgconvert

# Cross-compile targets: OS/ARCH pairs
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64

GO      := go
GOFMT   := gofmt
GOLINT  := golangci-lint
GINKGO  := $(GO) run github.com/onsi/ginkgo/v2/ginkgo

.PHONY: all build test lint fmt vet clean cross-compile $(PLATFORMS)

all: build

## Build both binaries for the current platform
build: $(GAME_BIN) $(EXTRACT_BIN) $(CONVERT_BIN)

$(GAME_BIN): cmd/moo2hd/main.go
	@mkdir -p bin
	$(GO) build -o $@ ./cmd/moo2hd

$(EXTRACT_BIN): cmd/lbxextract/main.go
	@mkdir -p bin
	$(GO) build -o $@ ./cmd/lbxextract

$(CONVERT_BIN): cmd/svgconvert/main.go
	@mkdir -p bin
	$(GO) build -o $@ ./cmd/svgconvert

## Run all tests via Ginkgo
test:
	$(GINKGO) -r --label-filter="!slow" ./...

## Run tests including slow/integration suites
test-all:
	$(GINKGO) -r ./...

## Format source files
fmt:
	$(GOFMT) -w .

## Run go vet
vet:
	$(GO) vet ./...

## Run golangci-lint (requires golangci-lint in PATH)
lint: vet
	$(GOLINT) run ./...

## Cross-compile for all target platforms
cross-compile: $(PLATFORMS)

$(PLATFORMS):
	$(eval OS   := $(word 1,$(subst /, ,$@)))
	$(eval ARCH := $(word 2,$(subst /, ,$@)))
	$(eval EXT  := $(if $(filter windows,$(OS)),.exe,))
	@mkdir -p dist/$(OS)_$(ARCH)
	GOOS=$(OS) GOARCH=$(ARCH) $(GO) build -o dist/$(OS)_$(ARCH)/moo2hd$(EXT)    ./cmd/moo2hd
	GOOS=$(OS) GOARCH=$(ARCH) $(GO) build -o dist/$(OS)_$(ARCH)/lbxextract$(EXT) ./cmd/lbxextract
	GOOS=$(OS) GOARCH=$(ARCH) $(GO) build -o dist/$(OS)_$(ARCH)/svgconvert$(EXT) ./cmd/svgconvert

clean:
	rm -rf bin/ dist/
