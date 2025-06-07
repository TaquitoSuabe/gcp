FROM golang:1.24-alpine AS builder
WORKDIR /app
RUN echo "module ssh-proxy" > go.mod && \
    echo "go 1.24" >> go.mod
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o proxy main.go
FROM debian:12-slim
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && \
    apt-get install -y \
    dropbear-run \
    tmux \
    net-tools \
    && apt-get clean && \
    rm -rf /var/lib/apt/lists/*
RUN useradd -m -s /bin/bash buhonero && \
    echo 'buhonero:gpc-test' | chpasswd
RUN mkdir -p /etc/dropbear && \
    dropbearkey -t rsa -f /etc/dropbear/dropbear_rsa_host_key && \
    dropbearkey -t ecdsa -f /etc/dropbear/dropbear_ecdsa_host_key
COPY --from=builder /app/proxy /usr/local/bin/proxy
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/proxy /usr/local/bin/entrypoint.sh
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
