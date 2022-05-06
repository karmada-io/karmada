FROM golang:1.17 AS builder

WORKDIR /go/src/github.com/karmada-io/karmada
COPY . /go/src/github.com/karmada-io/karmada

RUN make karmada-scheduler
RUN mv /go/src/github.com/karmada-io/karmada/karmada-scheduler /bin/


FROM alpine:3.15.1

RUN apk add --no-cache ca-certificates

COPY --from=builder /bin/karmada-scheduler /bin/karmada-scheduler

CMD ["/bin/karmada-scheduler"]

