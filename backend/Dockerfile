FROM golang:1.25-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build \
    -tags 'osusergo netgo sqlite_omit_load_extension' \
    -ldflags '-linkmode external -extldflags "-static" -s -w' \
    -o /pushkit-server ./cmd/server

FROM alpine:3.24
RUN apk add --no-cache ca-certificates
WORKDIR /data
COPY --from=builder /pushkit-server /usr/local/bin/pushkit-server
EXPOSE 8000
ENTRYPOINT ["pushkit-server"]
