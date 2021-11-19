FROM alpine:3.7

RUN apk add --no-cache ca-certificates

ADD karmada-interpreter-webhook-example /bin/

CMD ["/bin/karmada-interpreter-webhook-example"]
