FROM registry.access.redhat.com/ubi8/ubi

RUN yum install -y  gcc-c++ && yum clean all

ENV GOLANG_VERSION 1.10.3

RUN curl https://storage.googleapis.com/golang/go1.10.3.linux-amd64.tar.gz | tar -C /usr/local -xz

ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

WORKDIR /go/src/nvidia-device-plugin
COPY . .

RUN export CGO_LDFLAGS_ALLOW='-Wl,--unresolved-symbols=ignore-in-object-files' && \
    go install -ldflags="-s -w" -v nvidia-device-plugin

FROM registry.access.redhat.com/ubi8/ubi-minimal

ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=utility

COPY --from=build /go/bin/nvidia-device-plugin /usr/bin/nvidia-device-plugin

CMD ["nvidia-device-plugin"]
