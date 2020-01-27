FROM ubuntu:16.04

ARG DCGM_VERSION=1.7.1

RUN apt-get update && apt-get install -y --no-install-recommends libgomp1 \
    wget && \
    rm -rf /var/lib/apt/lists/*
	

RUN wget --no-check-certificate https://developer.download.nvidia.com/compute/redist/dcgm/${DCGM_VERSION}/DEBS/datacenter-gpu-manager_${DCGM_VERSION}_amd64.deb && \
    dpkg -i datacenter-gpu-manager_*.deb && \
    rm -f datacenter-gpu-manager_*.deb

COPY dcgm-exporter /usr/local/bin

ENV NVIDIA_VISIBLE_DEVICES all
ENV NVIDIA_DRIVER_CAPABILITIES all

VOLUME /run/prometheus

ENTRYPOINT [ "dcgm-exporter", "-e" ]
