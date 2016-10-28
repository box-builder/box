PACKAGES := "./cli-tests ./builder"

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
	@echo you must build the project with `make` before using this target
	go run main.go build.rb

run-test:
	docker run -e "TESTRUN=$(TESTRUN)" -it --privileged --rm -it box-test

test: build run-test

release: build
	docker run -e RELEASE=1 -it --privileged --rm -it box-test

docker-test:
	bash docker-test.sh $(PACKAGES)

.PHONY: docs
