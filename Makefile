all: vendor
	go install -v .

vendor:
	cd vendor/github.com/mitchellh/go-mruby && make clean all

.PHONY: vendor
