FROM alpine:3.17.1

ARG BINARY
ARG TARGETPLATFORM

RUN apk add --no-cache ca-certificates

COPY ${TARGETPLATFORM}/${BINARY} /bin/${BINARY}
