SUM := $(shell head -c 16 /dev/urandom | sha256sum | awk '{ print $$1 }' | tail -c 16)
PACKAGES := ./cli-tests ./layers ./image ./tar ./multi ./builder/executor/docker ./builder
BUILD_TAGS := "btrfs_noversion libdm_no_deferred_remove"

all: checks install

vndr:
	go get -u github.com/LK4D4/vndr
	vndr --whitelist go-mruby

fetch:
	cd vendor/github.com/mitchellh/go-mruby && MRUBY_CONFIG=$(shell pwd)/mruby_config.rb make

install: fetch
	go install -v -ldflags="-X main.Version=$${VERSION:-$(shell git rev-parse HEAD)}" -tags $(BUILD_TAGS) .

install-static: fetch
	go install -v -ldflags="-X main.Version=$${VERSION:-$(shell git rev-parse HEAD)} -extldflags=-static" -tags $(BUILD_TAGS) .

clean:
	cd vendor/github.com/mitchellh/go-mruby && make clean
	rm -rf bin

docs:
	mkdocs gh-deploy --clean

bootstrap:
	docker run --rm -i -w ${PWD} -v /var/run/docker.sock:/var/run/docker.sock -v ${PWD}:${PWD} box-builder/box:latest /dev/stdin < build.rb

bootstrap-test: bootstrap run-test

checks: fetch
	@sh checks.sh
 
build:
	SUM=${SUM} go run -tags $(BUILD_TAGS) main.go build.rb
 
build-ci:
	SUM=${SUM} CI_BUILD=1 go run -tags $(BUILD_TAGS) main.go --no-tty build.rb

run-test-ci:
	docker run -e "TESTRUN=$(TESTRUN)" --privileged --rm -i box-test-${SUM}

run-test:
	docker run -e "TESTRUN=$(TESTRUN)" --privileged --rm -it box-test-${SUM}

rmi:
	docker rmi box-test-${SUM}

test-ci: checks build-ci run-test-ci rmi

test: checks all build run-test rmi

release: clean all test
	VERSION=${VERSION} RELEASE=1 go run main.go -n -t box-builder/box:${VERSION} build.rb
	docker rm -f box-build-${VERSION} || :
	docker run --name box-build-${VERSION} --entrypoint /bin/bash box-builder/box:${VERSION} -c 'exit 0'
	docker cp box-build-${VERSION}:/box .
	docker rm box-build-${VERSION}
	sh release/release.sh ${VERSION}
	@echo File to release is RELEASE.tmp.md

test-local: clean all
	for i in $(PACKAGES); do go test -v $$i -check.vv; done

docker-test:
	/bin/bash docker-test.sh $(PACKAGES)

.PHONY: docs
