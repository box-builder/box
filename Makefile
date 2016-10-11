vendor:
	cd vendor/github.com/mitchellh/go-mruby && make

all: vendor
	go install -v .

.PHONY: vendor
