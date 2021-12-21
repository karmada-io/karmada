FROM alpine:3.7

RUN apk add --no-cache ca-certificates

ADD karmada-aggregated-apiserver /bin/

CMD ["/bin/karmada-aggregated-apiserver"]
