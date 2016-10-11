FROM golang
RUN apt-get update && apt-get install -y build-essential g++ git wget curl ruby bison flex
RUN mkdir -p /go/src/github.com/mitchellh
ENV GOPATH=/go
COPY . /go/src/github.com/erikh/box
RUN cd /go/src/github.com/erikh/box && \
    make

ENTRYPOINT "/go/bin/box"
