# Copyright (c) 2019-2022, NVIDIA CORPORATION.  All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM golang:1.25.5 AS base

# The devel image is used for local development.
FROM base AS devel

WORKDIR /work
COPY * .

ARG GOPROXY="https://goproxy.cn,direct"
ENV GOPROXY=$GOPROXY

RUN make install-tools

# We need to set the /work directory as a safe directory.
# This allows git commands to run in the container.
RUN git config --file=/.gitconfig --add safe.directory /work

# The build image is used to build the device plugin applications.
FROM base AS build

WORKDIR /build
COPY . .

RUN mkdir /artifacts
ARG VERSION="N/A"
ARG GIT_COMMIT="unknown"
RUN make PREFIX=/artifacts cmds

FROM nvcr.io/nvidia/distroless/go:v3.2.1-dev AS application

USER 0:0
SHELL ["/busybox/sh", "-c"]
RUN ln -s /busybox/sh /bin/sh

ENV NVIDIA_DISABLE_REQUIRE="true"
ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=compute,utility

ARG VERSION="N/A"
ARG GIT_COMMIT="unknown"

LABEL io.k8s.display-name="NVIDIA Device Plugin"
LABEL name="NVIDIA Device Plugin"
LABEL vendor="NVIDIA"
LABEL version=${VERSION}
LABEL com.nvidia.git-commit=${GIT_COMMIT}
LABEL release="N/A"
LABEL summary="NVIDIA device plugin for Kubernetes"
LABEL description="See summary"

COPY LICENSE /licenses/

COPY --from=build /artifacts/config-manager         /usr/bin/config-manager
COPY --from=build /artifacts/gpu-feature-discovery  /usr/bin/gpu-feature-discovery
COPY --from=build /artifacts/mps-control-daemon     /usr/bin/mps-control-daemon
COPY --from=build /artifacts/nvidia-device-plugin   /usr/bin/nvidia-device-plugin

ENTRYPOINT ["nvidia-device-plugin"]
