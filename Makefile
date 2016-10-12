all: vendor
	go install -v .

vendor:
	cd vendor/github.com/mitchellh/go-mruby && make clean all
	cp vendor/github.com/mitchellh/go-mruby/libmruby.a .

bootstrap:
	docker build -t box-bootstrap .

test:
	docker run -v /var/run/docker.sock:/var/run/docker.sock -i box-bootstrap < build.rb
	docker run -it --privileged --rm -it box-test make docker-test

release:
	docker run -v /var/run/docker.sock:/var/run/docker.sock -e RELEASE=1 -i box-bootstrap < build.rb

docker-test:
	bash docker-test.sh

.PHONY: vendor
