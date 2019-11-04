#docker build -f ../docker_build/Dockerfile -t amqpAdapter --target runtime  ../..

FROM alpine:latest
LABEL description="Runtime container - GooglePubSubAdapter"

#Copy the GooglePubSubAdapter binary into the image
COPY ./GooglePubSubAdapter /usr/local/bin/GooglePubSubAdapter

RUN apk --no-cache --update add ca-certificates

ENTRYPOINT ["/usr/local/bin/GooglePubSubAdapter"]