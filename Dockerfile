FROM golang:1.14.6-buster AS build

ADD . /go/src/github.com/github/emissary
WORKDIR /go/src/github.com/github/emissary

RUN apt-get update
RUN apt-get install -y curl

RUN CGO_ENABLED=0 go install -v ./cmd/emissary

FROM debian:stable-slim
COPY --from=build /go/bin/emissary /emissary

CMD ["/emissary"]
EXPOSE 9090
