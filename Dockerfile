# Use the official Golang image (stable version)
FROM golang:1.24-alpine AS builder

WORKDIR /app

ARG SERVICE_PORT=${SERVICE_PORT}
ARG SERVICE_PATH=${SERVICE_PATH}

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN cd ${SERVICE_PATH}

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build --ldflags="-w -s" -o main ${SERVICE_PATH}/main.go

FROM alpine:3.19

WORKDIR /app

COPY --from=builder /app/${SERVICE_PATH} .

EXPOSE ${SERVICE_PORT}

ENTRYPOINT ["./main"]