FROM ubuntu:16.04 as build

RUN apt-get update && apt-get install -y --no-install-recommends \
        g++ \
        ca-certificates \
        git wget \
        software-properties-common build-essential automake libtool && \
        add-apt-repository -y ppa:deadsnakes/ppa && \
    rm -rf /var/lib/apt/lists/*

ENV GOLANG_VERSION 1.12.7
ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=utility
RUN wget -nv -O - https://storage.googleapis.com/golang/go${GOLANG_VERSION}.linux-amd64.tar.gz \
    | tar -C /usr/local -xz
ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

COPY nvml.h /usr/local/include/
RUN ln -s /usr/lib/x86_64-linux-gnu/libnvidia-ml.so.1 /usr/lib/x86_64-linux-gnu/libnvidia-ml.so && \
    git clone https://github.com/open-mpi/hwloc && cd hwloc && ./autogen.sh && \
    ./configure CPPFLAGS=" -L/usr/lib/x86_64-linux-gnu" CFLAGS="-L/usr/lib/x86_64-linux-gnu" && \
    make && make install

WORKDIR /go/src/github.com/NVIDIA/k8s-device-plugin
COPY . .

ENV LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/usr/lib/x86_64-linux-gnu:/usr/local/lib
ENV LIBRARY_PATH=$LIBRARY_PATH:/usr/lib/x86_64-linux-gnu:/usr/local/lib
RUN export CGO_LDFLAGS_ALLOW='-Wl,--unresolved-symbols=ignore-in-object-files' && go build -ldflags="-s -w" .

CMD ["./k8s-device-plugin"]
