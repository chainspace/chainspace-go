FROM golang:alpine AS build

RUN echo $GOPATH

COPY . /go/src/chainspace.io/prototype
RUN CGO_ENABLED=0 GOOS=linux go install -a -tags netgo -ldflags '-w' chainspace.io/prototype/examples/cs-coin/cmd/cs-coin

FROM scratch

COPY --from=build /go/bin/cs-coin /cs-coin

ENTRYPOINT ["/cs-coin"]
