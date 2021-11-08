# Copyright (c) 2019-2021, NVIDIA CORPORATION.  All rights reserved.
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

ARG GOLANG_VERSION=1.15.8
ARG CUDA_IMAGE=cuda
ARG CUDA_VERSION=11.4.2
ARG BASE_DIST=ubuntu20.04
FROM golang:${GOLANG_VERSION} as build

WORKDIR /build
COPY . .

ARG PLUGIN_VERSION="N/A"
RUN export CGO_LDFLAGS_ALLOW='-Wl,--unresolved-symbols=ignore-in-object-files' && \
    go build -ldflags="-s -w -X 'main.Version=${PLUGIN_VERSION}'" -v ./cmd/nvidia-device-plugin


FROM nvcr.io/nvidia/${CUDA_IMAGE}:${CUDA_VERSION}-base-${BASE_DIST}

ENV NVIDIA_DISABLE_REQUIRE="true"
ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=utility

LABEL io.k8s.display-name="NVIDIA Device Plugin"
LABEL name="NVIDIA Device Plugin"
LABEL vendor="NVIDIA"
ARG PLUGIN_VERSION="N/A"
LABEL version=${PLUGIN_VERSION}
LABEL release="N/A"
LABEL summary="NVIDIA device plugin for Kubernetes"
LABEL description="See summary"

COPY ./LICENSE ./licenses/LICENSE

COPY --from=build /build/nvidia-device-plugin /usr/bin/nvidia-device-plugin

ENTRYPOINT ["nvidia-device-plugin"]
