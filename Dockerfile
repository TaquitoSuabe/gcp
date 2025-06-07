FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/app .
FROM debian:12-slim
RUN apt-get update && \
    apt-get install -y dropbear-run && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*
RUN useradd -m -s /bin/bash buhonero
RUN echo 'buhonero:gpc-test' | chpasswd
USER root 
WORKDIR /
COPY --from=builder /app/app /usr/local/bin/app
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
