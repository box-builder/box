FROM golang
RUN apt-get update && apt-get install -y build-essential g++ git wget curl ruby bison flex
RUN mkdir -p /go/src/github.com/mitchellh
ENV GOPATH=/go
RUN go get github.com/docker/engine-api && \
    go get github.com/docker/distribution/reference && \
    go get github.com/docker/go-connections/nat && \
    go get github.com/docker/go-units && \
    go get golang.org/x/net/context && \
    go get github.com/Sirupsen/logrus && \
    go get github.com/opencontainers/runc/libcontainer/user 
RUN cd /go/src/github.com/mitchellh && \
    git clone https://github.com/erikh/go-mruby && \
    cd go-mruby && \
    git fetch && \
    git checkout -b class origin/class
RUN cd /go/src/github.com/mitchellh/go-mruby && \
    make && \
    cp libmruby.a /root/
RUN cd /root && \
    go get -v github.com/erikh/box

ENTRYPOINT "/go/bin/box"
