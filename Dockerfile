FROM golang:latest as build-env

WORKDIR /go/src/app
ADD https://raw.githubusercontent.com/koceg/azure/master/vcpu.go /go/src/app

RUN go get -d -v ./...

RUN rm /go/bin/app
RUN go build -o /go/bin/app

FROM gcr.io/distroless/base
COPY --from=build-env /go/bin/app /
CMD ["/app"]
