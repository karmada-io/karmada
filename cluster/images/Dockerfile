FROM alpine:3.18.2

ARG BINARY

RUN apk add --no-cache ca-certificates
#tzdata is used to parse the time zone information when using CronFederatedHPA
RUN apk add --no-cache tzdata

COPY ${BINARY} /bin/${BINARY}
