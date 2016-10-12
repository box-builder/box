all: vendor
	go install -v .

vendor:
	cd vendor/github.com/mitchellh/go-mruby && make clean all
	cp vendor/github.com/mitchellh/go-mruby/libmruby.a .

test:
	docker build -t box .
	docker run -it --privileged --rm -it box make docker-test

docker-test:
	bash docker-test.sh

.PHONY: vendor
