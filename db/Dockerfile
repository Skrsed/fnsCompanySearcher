FROM alpine:latest

WORKDIR /var/lib/sqlite

RUN apk update
RUN apk add sqlite sqlite-dev

CMD ["./init.sh"]