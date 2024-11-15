FROM golang:1.23.2-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o myapp .

FROM alpine:3.18

COPY --from=builder /app/myapp /myapp

ENV USERAPI_CACHING_ADDRESS=redis:5000
ENV USERAPI_CACHING_PASSWORD=
ENV USERAPI_CACHING_DB=0

ENV NEWS_CACHING_ADDRESS=redis:6000
ENV NEWS_CACHING_PASSWORD=
ENV NEWS_CACHING_DB=0

EXPOSE 8080

ENTRYPOINT ["/myapp"]
