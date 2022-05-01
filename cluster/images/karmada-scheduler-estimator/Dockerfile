FROM golang:1.17 AS builder

WORKDIR /go/src/github.com/karmada-io/karmada
COPY . /go/src/github.com/karmada-io/karmada

RUN make karmada-scheduler-estimator
RUN mv /go/src/github.com/karmada-io/karmada/karmada-scheduler-estimator /bin/


FROM alpine:3.15.1

RUN apk add --no-cache ca-certificates

COPY --from=builder /bin/karmada-scheduler-estimator /bin/karmada-scheduler-estimator

CMD ["/bin/karmada-scheduler-estimator"]

