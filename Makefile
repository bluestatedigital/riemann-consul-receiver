NAME=riemann-consul-receiver
## tags are like v1.0.0
VER=$(shell git describe --always --dirty | sed -e 's/^v//g' )
BIN=.godeps/bin

GPM=$(BIN)/gpm
GPM_LINK=$(BIN)/gpm-link
GVP=$(BIN)/gvp

## @todo should use "$(GVP) in", but that fails
SOURCES=$(shell go list -f '{{range .GoFiles}}{{.}} {{end}}' . )
TEST_SOURCES=$(shell go list -f '{{range .TestGoFiles}}{{ $$.Dir }}/{{.}} {{end}}' . | sed -e "s@$(PWD)/@@g" )

.PHONY: all devtools deps test build clean rpm

## targets after a | are order-only; the presence of the target is sufficient
## http://stackoverflow.com/questions/4248300/in-a-makefile-is-a-directory-name-a-phony-target-or-real-target

all: build

$(BIN) stage:
	mkdir -p $@

$(GPM): | $(BIN)
	curl -s -L -o $@ https://github.com/pote/gpm/raw/v1.3.1/bin/gpm
	chmod +x $@

$(GPM_LINK): | $(BIN)
	curl -s -L -o $@ https://github.com/elcuervo/gpm-link/raw/v0.0.1/bin/gpm-link
	chmod +x $@

$(GVP): | $(BIN)
	curl -s -L -o $@ https://github.com/pote/gvp/raw/v0.1.0/bin/gvp
	chmod +x $@

.godeps/.gpm_installed: $(GPM) $(GVP) $(GPM_LINK) Godeps
	$(GVP) in $(GPM) link add github.com/bluestatedigital/riemann-consul-receiver $(PWD)
	$(GVP) in $(GPM) install
	touch $@

$(BIN)/ginkgo: .godeps/.gpm_installed
	$(GVP) in go install github.com/onsi/ginkgo/ginkgo
	touch $@

$(BIN)/mockery: .godeps/.gpm_installed
	$(GVP) in go install github.com/vektra/mockery
	touch $@

## installs dev tools
devtools: $(BIN)/ginkgo $(BIN)/mockery

## just installs dependencies
deps: .godeps/.gpm_installed

## run tests
test: $(BIN)/ginkgo $(TEST_SOURCES)
	$(GVP) in $(BIN)/ginkgo

## build the binary
## augh!  gvp shell escaping!!
## https://github.com/pote/gvp/issues/22
stage/$(NAME): .godeps/.gpm_installed $(SOURCES) | stage
	$(GVP) in go build -o $@ -ldflags '-X\ main.version\ $(VER)' -v .

## same, but shorter
build: test stage/$(NAME)

## duh
clean:
	rm -rf stage .godeps release

rpm: build
	mkdir -p stage/rpm/usr/bin stage/rpm/etc/logrotate.d stage/rpm/etc/sysconfig stage/rpm/etc/rc.d/init.d
	
	cp stage/$(NAME) stage/rpm/usr/bin/
	
	## config file
	cp etc/sysconfig stage/rpm/etc/sysconfig/$(NAME)
	cp etc/riemann-consul-receiver.logrotate stage/rpm/etc/logrotate.d/$(NAME)
	
	## init script
	cp etc/sysvinit.sh stage/rpm/etc/rc.d/init.d/$(NAME)
	chmod 555 stage/rpm/etc/rc.d/init.d/$(NAME)

	cd stage && fpm \
	    -s dir \
	    -t rpm \
	    -n $(NAME) \
	    -v $(VER) \
	    --rpm-use-file-permissions \
	    --config-files /etc/sysconfig/$(NAME) \
	    -C rpm \
	    etc usr
