FROM golang:1.23.6 AS builder
ARG ENV_NAME
ARG APP_NAME=id-hash
WORKDIR /usr/src/${APP_NAME}

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod vendor -ldflags "\
    -X gitlab.adtelligent.com/common/shared/log.buildVersion=$(git rev-list --count $(git rev-parse HEAD)) \
    -X gitlab.adtelligent.com/common/shared/log.buildTime=$(date -u '+%Y-%m-%d_%H:%M:%S') \
    -X gitlab.adtelligent.com/common/shared/log.buildRevision=$(git rev-parse HEAD)" \
    -a -installsuffix cgo -o /usr/local/bin/${APP_NAME} ./cmd/${APP_NAME}

# Final stage
FROM alpine
ARG APP_NAME=id-hash
COPY --from=builder /usr/local/bin/${APP_NAME} /usr/local/bin/${APP_NAME}

EXPOSE 8080
EXPOSE 11813

CMD ["/usr/local/bin/id-hash", "-config", "/etc/id-hash/base.ini"]
