FROM golang
RUN apt-get update && apt-get install -y build-essential g++ git wget curl ruby bison flex iptables psmisc
ENV GOPATH=/go
RUN wget https://get.docker.com/builds/Linux/x86_64/docker-1.12.1.tgz && \
    tar -xpf docker-1.12.1.tgz --strip-components=1 -C /usr/bin/ && \
    rm docker-1.12.1.tgz
COPY . /go/src/github.com/erikh/box
RUN cp /go/src/github.com/erikh/box/dind /dind
RUN cd /go/src/github.com/erikh/box && \
    make

ENTRYPOINT ["/dind"]
