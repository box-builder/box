PACKAGES := "./cli-tests ./builder ./builder/executor/docker ./image"

all:
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
 
build:
	go run main.go build.rb

run-test:
	docker run -e "TESTRUN=$(TESTRUN)" -it --privileged --rm -it box-test

test: build run-test

release: clean all test
	RELEASE=1 go run main.go -t erikh/box:${VERSION} build.rb
	sh release/release.sh ${VERSION}
	@echo File to release is RELEASE.tmp.md

release-osx: clean all
	# test directly on mac
	go test -v ./cli-tests -check.vv
	go test -v ./builder -check.vv
	sh release/release.sh ${VERSION}

docker-test:
	bash docker-test.sh $(PACKAGES)

.PHONY: docs
