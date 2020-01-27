FROM golang:1.11 as build

WORKDIR /go/src/pod-gpu-metrics-exporter
COPY src .

RUN go install -v pod-gpu-metrics-exporter

FROM debian:stretch-slim

COPY --from=build /go/bin/pod-gpu-metrics-exporter /usr/bin/pod-gpu-metrics-exporter

ENTRYPOINT ["pod-gpu-metrics-exporter", "-logtostderr"]
