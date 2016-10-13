all: vendor
	go install -v .

vendor:
	cd vendor/github.com/mitchellh/go-mruby && make clean all
	cp vendor/github.com/mitchellh/go-mruby/libmruby.a .

bootstrap:
	docker build -t box-bootstrap .

bootstrap-test: bootstrap
	docker run -v /var/run/docker.sock:/var/run/docker.sock -i box-bootstrap < build.rb
	make test
 
build:
	go run main.go < build.rb

test: build
	docker run -it --privileged --rm -it box-test make docker-test

release: build
	docker run -it --privileged --rm -it box-test make docker-test

docker-test:
	bash docker-test.sh

.PHONY: vendor
