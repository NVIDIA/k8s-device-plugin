#!/bin/bash
cp -f /etc/vgpu/* /usr/local/vgpu/
exec nvidia-device-plugin $@