PACKAGES := ./cli-tests ./builder ./builder/executor/docker ./layers ./image ./tar ./multi
BUILD_TAGS := "btrfs_noversion libdm_no_deferred_remove"

all: checks install

fetch:
	cd vendor/github.com/mitchellh/go-mruby && MRUBY_CONFIG=$(shell pwd)/mruby_config.rb make

install: fetch
	go install -v -tags $(BUILD_TAGS) .

clean:
	cd vendor/github.com/mitchellh/go-mruby && make clean

docs:
	mkdocs gh-deploy --clean

bootstrap-ci:
	docker run --rm -e "CI_BUILD=1" -i -w ${PWD} -v /var/run/docker.sock:/var/run/docker.sock -v ${PWD}:${PWD} erikh/box:latest /dev/stdin < build.rb

bootstrap:
	docker run --rm -ti -w ${PWD} -v /var/run/docker.sock:/var/run/docker.sock -v ${PWD}:${PWD} erikh/box:latest /dev/stdin < build.rb

checks: fetch
	@sh checks.sh
 
build:
	go run -tags $(BUILD_TAGS) main.go build.rb
 
build-ci:
	CI_BUILD=1 go run -tags $(BUILD_TAGS) main.go --no-tty build.rb

test-ci: checks bootstrap-ci
	docker run -e "TESTRUN=$(TESTRUN)" --privileged --rm -i box-test

test: checks bootstrap
	docker run -e "TESTRUN=$(TESTRUN)" --privileged --rm -it box-test

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
