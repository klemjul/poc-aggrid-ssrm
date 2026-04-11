FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /backend .

FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /backend /backend
EXPOSE 8080
CMD ["/backend"]
