FROM golang:1.22-alpine as builder

COPY src /src
WORKDIR /src
RUN go mod download
RUN go build -o ./bin/api

FROM alpine:latest
COPY --from=builder /src/bin/api /bin/api
CMD ["/bin/api"]