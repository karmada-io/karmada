FROM alpine:3.19.1

ARG BINARY

RUN apk add --no-cache ca-certificates

COPY ${BINARY} /bin/${BINARY}
