FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /go/bin
COPY ./dist/dm-repo-integration_linux_amd64_v1/dm-repo-integration ./app
COPY config/application.production.properties ./application.properties
ENV C8Y_LOGGER_HIDE_SENSITIVE=true
CMD ["./app"]
