FROM ubuntu:16.04

# Build:    docker build -t dingo .
# Run:      docker run -it --rm dingo
# Run args: docker run -it --rm dingo /bin/bash -c "dgc FILES && ./dgexe [ARGS]"

RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    curl \
    python \
    subversion \
    && rm -rf /var/lib/apt/lists/*

# Download and install Go 1.6.2
RUN curl https://dl.google.com/go/go1.6.2.linux-amd64.tar.gz | tar -C /usr/local -xz

ENV GOPATH /go
ENV PATH $PATH:/usr/local/go/bin:$GOPATH/bin
ENV GOLLVM $GOPATH/src/llvm.org/llvm

# Download and install LLVM 6.0 and bindings
RUN svn co http://llvm.org/svn/llvm-project/llvm/tags/RELEASE_600/final $GOLLVM \
    && $GOLLVM/bindings/go/build.sh -DCMAKE_BUILD_TYPE=Release -DLLVM_TARGETS_TO_BUILD=host \
    && go install llvm.org/llvm/bindings/go/llvm

WORKDIR $GOPATH/src/github.com/cjo5/dingo

# Copy source code
COPY internal ./internal
COPY cmd ./cmd

# Install compiler and test tool
RUN go install github.com/cjo5/dingo/cmd/dgc \
    && go install github.com/cjo5/dingo/cmd/dgc-test

# Copy Dingo tests and examples
# Files specified as arguments to docker run should be located in these directories
COPY std ./std
COPY test ./test
COPY examples/ ./examples

CMD ["/bin/bash", "-c", "dgc test/docker_hello.dg && ./dgexe"]
