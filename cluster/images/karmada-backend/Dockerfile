FROM alpine:3.7

RUN apk add --no-cache ca-certificates

RUN mkdir /root/.kube

COPY karmada-backend /bin/

CMD ["/bin/karmada-backend"]
