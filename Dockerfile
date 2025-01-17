FROM golang AS builder

ARG CGO_ENABLED=0

WORKDIR /go/src/app
ADD . /go/src/app

RUN go mod download
RUN go build -o /go/bin/doras-server github.com/unbasical/doras-server/cmd/doras-server

FROM gcr.io/distroless/base AS build
ARG PORT=8080

COPY --from=builder /go/bin/doras-server /
EXPOSE $PORT
ENTRYPOINT ["/doras-server"]