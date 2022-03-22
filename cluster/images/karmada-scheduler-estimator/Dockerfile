FROM alpine:3.15.1

RUN apk add --no-cache ca-certificates

ADD karmada-scheduler-estimator /bin/

CMD ["/bin/karmada-scheduler-estimator"]
