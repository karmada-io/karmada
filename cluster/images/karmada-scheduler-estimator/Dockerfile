FROM alpine:3.7

RUN apk add --no-cache ca-certificates

ADD karmada-scheduler-estimator /bin/

CMD ["/bin/karmada-scheduler-estimator"]
