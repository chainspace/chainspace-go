FROM golang:alpine AS build

COPY . /go/src/chainspace.io/dummychecker
WORKDIR /go/src/chainspace.io/dummychecker
RUN CGO_ENABLED=0 GOOS=linux go build -a -tags netgo -ldflags '-w'

FROM scratch

COPY --from=build /go/src/chainspace.io/dummychecker/dummychecker /dummychecker

ENTRYPOINT ["/dummychecker"]
