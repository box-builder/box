PACKAGES := "./cli-tests ./builder ./builder/executor/docker ./image ./tar"

all: checks install

install:
	cd vendor/github.com/mitchellh/go-mruby && MRUBY_CONFIG=$(shell pwd)/mruby_config.rb make
	go install -v .

clean:
	cd vendor/github.com/mitchellh/go-mruby && make clean

docs:
	mkdocs gh-deploy --clean

bootstrap:
	docker build -t box-bootstrap .

bootstrap-image: bootstrap
	docker run -v /var/run/docker.sock:/var/run/docker.sock box-bootstrap box build.rb

bootstrap-test: bootstrap-image
	make run-test

checks:
	@sh checks.sh
 
build:
	go run main.go build.rb
 
build-ci:
	CI_BUILD=1 go run main.go --no-tty build.rb

run-test-ci:
	docker run -e "TESTRUN=$(TESTRUN)" --privileged --rm -i box-test

run-test:
	docker run -e "TESTRUN=$(TESTRUN)" --privileged --rm -it box-test

test-ci: checks build-ci run-test-ci

test: checks build run-test

release: clean all test
	RELEASE=1 go run main.go -t erikh/box:${VERSION} build.rb
	sh release/release.sh ${VERSION}
	@echo File to release is RELEASE.tmp.md

release-osx: clean all
	# test directly on mac
	for i in $(PACKAGES); do go test -v $$i -check.vv; done
	sh release/release.sh ${VERSION}

docker-test:
	bash docker-test.sh $(PACKAGES)

.PHONY: docs
