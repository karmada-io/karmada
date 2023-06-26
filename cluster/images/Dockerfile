FROM alpine:3.18.2

ARG BINARY

RUN apk add --no-cache ca-certificates bind-tools

COPY ${BINARY} /bin/${BINARY}
