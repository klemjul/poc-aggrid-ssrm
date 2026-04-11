FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /backend .

FROM alpine:3.21
RUN addgroup -S app && adduser -S -G app app \
    && apk --no-cache add ca-certificates tzdata
COPY --from=builder /backend /backend
EXPOSE 8080
USER app
CMD ["/backend"]
