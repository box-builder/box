PACKAGES := ./cli-tests ./builder ./builder/executor/docker ./layers ./image ./tar ./multi

all: checks install

fetch:
	cd vendor/github.com/mitchellh/go-mruby && MRUBY_CONFIG=$(shell pwd)/mruby_config.rb make

install: fetch
	go install -t btrfs_noversion -v .

clean:
	cd vendor/github.com/mitchellh/go-mruby && make clean

docs:
	mkdocs gh-deploy --clean

bootstrap:
	docker run --rm -i -w ${PWD} -v /var/run/docker.sock:/var/run/docker.sock -v ${PWD}:${PWD} erikh/box:latest /dev/stdin < build.rb

bootstrap-test: bootstrap run-test

checks: fetch
	@sh checks.sh
 
build:
	go run -t btrfs_noversion main.go build.rb
 
build-ci:
	CI_BUILD=1 go run main.go --no-tty build.rb

run-test-ci:
	docker run -e "TESTRUN=$(TESTRUN)" --privileged --rm -i box-test

run-test:
	docker run -e "TESTRUN=$(TESTRUN)" --privileged --rm -it box-test

test-ci: checks build-ci run-test-ci

test: checks all build run-test

release: clean all test
	sh release/release.sh ${VERSION}
	RELEASE=1 go run main.go -t erikh/box:${VERSION} build.rb
	@echo File to release is RELEASE.tmp.md

test-local: clean all
	for i in $(PACKAGES); do go test -v $$i -check.vv; done

release-osx: test-local # test directly on mac
	sh release/release.sh ${VERSION}

docker-test:
	/bin/bash docker-test.sh $(PACKAGES)

.PHONY: docs
