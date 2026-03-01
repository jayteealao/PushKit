FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /pushkit-server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /pushkit-server /usr/local/bin/pushkit-server
EXPOSE 8000
ENTRYPOINT ["pushkit-server"]
