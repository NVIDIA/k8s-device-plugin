FROM --platform=ppc64le centos:7 as build

RUN yum install -y \
        gcc-c++ \
        ca-certificates \
        wget && \
    rm -rf /var/cache/yum/*

ENV GOLANG_VERSION 1.10.3
RUN wget -nv -O - https://storage.googleapis.com/golang/go${GOLANG_VERSION}.linux-ppc64le.tar.gz \
    | tar -C /usr/local -xz
ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

WORKDIR /go/src/nvidia-device-plugin
COPY . .

RUN export CGO_LDFLAGS_ALLOW='-Wl,--unresolved-symbols=ignore-in-object-files' && \
    go install -ldflags="-s -w" -v nvidia-device-plugin


FROM --platform=ppc64le centos:7

ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=utility

COPY --from=build /go/bin/nvidia-device-plugin /usr/bin/nvidia-device-plugin

CMD ["nvidia-device-plugin"]
