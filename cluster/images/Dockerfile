FROM alpine:3.17.1

ARG BINARY

RUN apk add --no-cache ca-certificates

COPY ${BINARY} /bin/${BINARY}
