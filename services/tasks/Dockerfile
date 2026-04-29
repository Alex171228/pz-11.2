FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/bin/tasks ./services/tasks/cmd/tasks

FROM alpine:3.20

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/bin/tasks /app/tasks

EXPOSE 8082

ENV TASKS_PORT=8082
ENV AUTH_MODE=http
ENV AUTH_BASE_URL=http://host.docker.internal:8081

CMD ["/app/tasks"]
