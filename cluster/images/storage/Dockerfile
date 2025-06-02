FROM ubuntu:22.04

WORKDIR /app

RUN apt-get update && apt-get install -y inotify-tools tini && \
    rm -rf /var/lib/apt/lists/*

COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

ENTRYPOINT ["tini", "--", "./entrypoint.sh"]
