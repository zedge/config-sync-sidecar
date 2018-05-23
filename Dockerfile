FROM alpine:latest
COPY bin/configsync /
ENTRYPOINT ["/configsync"]
