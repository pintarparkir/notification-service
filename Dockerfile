FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/notification ./cmd/notification

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /out/notification /notification
ENTRYPOINT ["/notification"]
