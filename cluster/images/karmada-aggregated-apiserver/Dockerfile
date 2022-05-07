FROM golang:1.17 AS builder

WORKDIR /go/src/github.com/karmada-io/karmada
COPY . /go/src/github.com/karmada-io/karmada

RUN make karmada-aggregated-apiserver
RUN mv /go/src/github.com/karmada-io/karmada/karmada-aggregated-apiserver /bin/


FROM alpine:3.15.1

RUN apk add --no-cache ca-certificates

COPY --from=builder /bin/karmada-aggregated-apiserver /bin/karmada-aggregated-apiserver

CMD ["/bin/karmada-aggregated-apiserver"]
