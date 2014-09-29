NAME=riemann-consul-receiver
VER=$(shell git describe --always --dirty )
BIN=.godeps/bin

GPM=$(BIN)/gpm
GVP=$(BIN)/gvp

## @todo should use "$(GVP) in", but that fails
SOURCES=$(shell go list -f '{{range .GoFiles}}{{.}} {{end}}' ./... )
TEST_SOURCES=$(shell go list -f '{{range .TestGoFiles}}{{.}} {{end}}' ./... )

.PHONY: all build tools clean release deps test

all: build

$(BIN) stage:
	mkdir -p $@

$(GPM): $(BIN)
	curl -s -L -o $@ https://github.com/pote/gpm/raw/v1.2.3/bin/gpm
	chmod +x $@

$(GVP): $(BIN)
	curl -s -L -o $@ https://github.com/pote/gvp/raw/v0.1.0/bin/gvp
	chmod +x $@

.godeps: $(GVP)
	$(GVP) init

.godeps/.gpm_installed: .godeps $(GPM) $(GVP) Godeps
	$(GVP) in $(GPM) install
	touch $@

.godeps/bin/ginkgo: .godeps/.gpm_installed
	$(GVP) in go install github.com/onsi/ginkgo/ginkgo

.godeps/bin/mockery: .godeps/.gpm_installed
	$(GVP) in go install github.com/vektra/mockery

## installs dev tools
devtools: .godeps/bin/ginkgo .godeps/bin/mockery

## just installs dependencies
deps: .godeps/.gpm_installed

## run tests
test: .godeps/bin/ginkgo $(TEST_SOURCES)
	$(GVP) in .godeps/bin/ginkgo

## build the binary
stage/$(NAME): .godeps/.gpm_installed stage $(SOURCES)
	## augh!  gvp shell escaping!!
	## https://github.com/pote/gvp/issues/22
	$(GVP) in go build -o $@ -ldflags '-X\ main.version\ $(VER)' -v ./...

## same, but shorter
build: stage/$(NAME)

## duh
clean:
	rm -rf stage .godeps release

release/$(NAME): $(SOURCES)
	docker run \
		-i -t \
		-v $(PWD):/gopath/src/app \
		-w /gopath/src/app \
		google/golang:1.3 \
		make clean test build
	
	mkdir -p release
	mv stage/$(NAME) $@

release: release/$(NAME)
