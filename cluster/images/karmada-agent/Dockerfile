FROM golang:1.17 AS builder

WORKDIR /go/src/github.com/karmada-io/karmada
COPY . /go/src/github.com/karmada-io/karmada

RUN make karmada-agent
RUN mv /go/src/github.com/karmada-io/karmada/karmada-agent /bin/


FROM alpine:3.15.1

RUN apk add --no-cache ca-certificates

COPY --from=builder /bin/karmada-agent /bin/karmada-agent

CMD ["/bin/karmada-agent"]
