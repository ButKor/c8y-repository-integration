FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /go/bin
COPY ./dist/repo-int_linux_amd64_v1/repo-int ./app
COPY config/application.production.properties ./application.properties
ENV C8Y_LOGGER_HIDE_SENSITIVE=true
CMD ["./app"]
