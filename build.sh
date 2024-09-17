#!/bin/bash

mkdir -p build

docker build -t kobo-uncaged:latest .
docker run -v "$PWD/build":/uncaged/build kobo-uncaged:latest
