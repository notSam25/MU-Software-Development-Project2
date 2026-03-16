ARG GOLANG_VERSION
FROM golang:${GOLANG_VERSION} AS builder

# Set necessary environment variables
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Set the working directory inside the container
WORKDIR /opt/webapp/

ENV TZ=UTC

RUN apk add --no-cache curl git && \
    curl -sSfL https://raw.githubusercontent.com/cosmtrek/air/master/install.sh | sh -s -- -b /usr/local/bin

CMD ["air"]