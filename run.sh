#!/bin/sh

cd /home/workspace/docker-tool && /usr/local/go/bin/go run main.go >> log/gorun.log 2>&1 &