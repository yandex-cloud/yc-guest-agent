FROM golang:1.16.10

COPY ./ /agent/
WORKDIR /agent