FROM alpine:3.17.1

ARG BINARY

RUN apk add --no-cache ca-certificates

RUN apk update && apk add bind-tools

COPY ${BINARY} /bin/${BINARY}
