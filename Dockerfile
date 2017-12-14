FROM nvidia/cuda:9.0-base-ubuntu16.04 as build

RUN apt-get update && apt-get install -y --no-install-recommends \
        g++ \
        ca-certificates \
        wget \
        cuda-cudart-dev-9-0 \
        cuda-misc-headers-9-0 \
        cuda-nvml-dev-9-0 && \
    rm -rf /var/lib/apt/lists/*

ENV GOLANG_VERSION 1.9.2
RUN wget -nv -O - https://storage.googleapis.com/golang/go${GOLANG_VERSION}.linux-amd64.tar.gz \
    | tar -C /usr/local -xz
ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

ENV CGO_CFLAGS "-I /usr/local/cuda-9.0/include"
ENV CGO_LDFLAGS "-L /usr/local/cuda-9.0/lib64"
ENV PATH=$PATH:/usr/local/nvidia/bin:/usr/local/cuda/bin

WORKDIR /go/src/nvidia-device-plugin
COPY . .

RUN go install -ldflags="-s -w" -v nvidia-device-plugin


FROM alpine:3.6

RUN apk --no-cache add ca-certificates wget && \
    wget -q -O /etc/apk/keys/sgerrand.rsa.pub https://raw.githubusercontent.com/sgerrand/alpine-pkg-glibc/master/sgerrand.rsa.pub && \
    wget https://github.com/sgerrand/alpine-pkg-glibc/releases/download/2.26-r0/glibc-2.26-r0.apk && \
    apk add glibc-2.26-r0.apk && \
    rm glibc-2.26-r0.apk && \
    apk del ca-certificates wget

ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=utility
ENV LD_LIBRARY_PATH=/usr/lib64

COPY --from=build /go/bin/nvidia-device-plugin /usr/bin/nvidia-device-plugin

CMD ["nvidia-device-plugin"]
