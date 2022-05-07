FROM golang:1.17 AS builder

WORKDIR /go/src/github.com/karmada-io/karmada
COPY . /go/src/github.com/karmada-io/karmada

RUN make karmada-interpreter-webhook-example
RUN mv /go/src/github.com/karmada-io/karmada/karmada-interpreter-webhook-example /bin/


FROM alpine:3.15.1

RUN apk add --no-cache ca-certificates

COPY --from=builder /bin/karmada-interpreter-webhook-example /bin/karmada-interpreter-webhook-example

CMD ["/bin/karmada-interpreter-webhook-example"]

