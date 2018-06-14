FROM ubuntu:16.04

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

# Download and install LLVM 6.0
RUN svn co http://llvm.org/svn/llvm-project/llvm/tags/RELEASE_600/final $GOLLVM
RUN $GOLLVM/bindings/go/build.sh -DCMAKE_BUILD_TYPE=Release -DLLVM_TARGETS_TO_BUILD=host 

# Install Go LLVM bindings
RUN go install llvm.org/llvm/bindings/go/llvm

ENV DINGO $GOPATH/src/github.com/jhnl/dingo
COPY . $DINGO

# Install Dingo compiler
RUN go install github.com/jhnl/dingo/cmd/dgc

# Install test tool
RUN go install github.com/jhnl/dingo/cmd/dgc-test

WORKDIR $DINGO
RUN printf '#!/usr/bin/env bash\ndgc $1 && ./dgexe\n' > entrypoint.sh && chmod +x entrypoint.sh

ENTRYPOINT [ "./entrypoint.sh" ]
CMD ["test/docker_hello.dg"]
