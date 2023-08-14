FROM alpine:3.18.3

ARG BINARY
ARG TARGETPLATFORM

RUN apk add --no-cache ca-certificates
#tzdata is used to parse the time zone information when using CronFederatedHPA
RUN apk add --no-cache tzdata

COPY ${TARGETPLATFORM}/${BINARY} /bin/${BINARY}
