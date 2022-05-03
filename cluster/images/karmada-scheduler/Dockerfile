FROM golang:1.17-alpine3.15 AS builder

WORKDIR /go/src/github.com/karmada-io/karmada
COPY . /go/src/github.com/karmada-io/karmada

ARG GO_LDFLAGS

RUN CGO_ENABLED=0 go build \
    -ldflags "$GO_LDFLAGS" \
    -o /bin/karmada-scheduler \
    cmd/scheduler/main.go


FROM alpine:3.15.1

RUN apk add --no-cache ca-certificates

COPY --from=builder /bin/karmada-scheduler /bin/karmada-scheduler

CMD ["/bin/karmada-scheduler"]

