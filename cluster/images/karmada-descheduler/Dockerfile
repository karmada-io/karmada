FROM alpine:3.15.1

RUN apk add --no-cache ca-certificates

ADD karmada-descheduler /bin/

CMD ["/bin/karmada-descheduler"]
