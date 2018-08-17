FROM golang:alpine AS build

LABEL maintainer "jeremy@chainspace.io"

RUN apk update && apk upgrade && apk add curl openssh git
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
COPY . /go/src/chainspace.io/prototype
WORKDIR /go/src/chainspace.io/prototype
RUN CGO_ENABLED=0 GOOS=linux go install -a -tags netgo -ldflags '-w' chainspace.io/prototype/cmd/chainspace

FROM scratch

COPY --from=build /go/bin/chainspace /chainspace

ENTRYPOINT ["/chainspace"]