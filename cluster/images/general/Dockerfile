FROM golang:1.17 as builder
COPY karmada /code/karmada
WORKDIR /code/karmada
RUN make _TARGET_

FROM alpine:3.7
RUN apk add -u --no-cache ca-certificates
COPY --from=builder "/code/karmada/_TARGET_" /bin/
CMD ["/bin/_TARGET_"]

