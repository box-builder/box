all: vendor
	go install -v .

vendor:
	cd vendor/github.com/mitchellh/go-mruby && make clean all
	cp vendor/github.com/mitchellh/go-mruby/libmruby.a .

test:
	dockerd -s vfs &
	sleep 5
	# replace this with the actual tests
	docker info
	killall dockerd
	wait

.PHONY: vendor
