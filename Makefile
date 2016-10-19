PACKAGES := "./builder"

all: libmruby.a 
	go install -v .

clean:
	rm -f libmruby.a box

docs:
	mkdocs gh-deploy --clean

libmruby.a:
	cd vendor/github.com/mitchellh/go-mruby && make clean all
	cp vendor/github.com/mitchellh/go-mruby/libmruby.a .

bootstrap:
	docker build -t box-bootstrap .

bootstrap-image: bootstrap
	docker run -v /var/run/docker.sock:/var/run/docker.sock -i box-bootstrap < build.rb

bootstrap-test: bootstrap-image
	make run-test
 
build: libmruby.a
	go run main.go < build.rb

run-test:
	docker run -it --privileged --rm -it box-test

test: build run-test

release: build
	docker run -e RELEASE=1 -it --privileged --rm -it box-test

docker-test:
	bash docker-test.sh $(PACKAGES)

.PHONY: vendor docs
