FROM golang
RUN apt-get update && apt-get install -y build-essential g++ git wget curl ruby bison flex iptables psmisc
ENV GOPATH=/go
COPY . /go/src/github.com/erikh/box
RUN cp /go/src/github.com/erikh/box/dind /dind
RUN cd /go/src/github.com/erikh/box && \
    make

WORKDIR /go/src/github.com/erikh/box
ENTRYPOINT "box"
