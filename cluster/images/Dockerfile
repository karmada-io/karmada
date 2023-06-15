FROM alpine:3.18.2

ARG BINARY

RUN apk add --no-cache ca-certificates

COPY ${BINARY} /bin/${BINARY}
