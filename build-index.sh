#!/usr/bin/env bash

helm repo index stable --url https://nvidia.github.io/k8s-device-plugin/stable

cp -f stable/index.yaml index.yaml