FROM alpine:3.15.1

RUN apk add --no-cache ca-certificates

ADD karmada-aggregated-apiserver /bin/

CMD ["/bin/karmada-aggregated-apiserver"]
