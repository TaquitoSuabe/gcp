FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/app .
FROM debian:12-slim
RUN apt-get update && \
    apt-get install -y openssh-server && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*
RUN mkdir /var/run/sshd
RUN useradd -m -s /bin/bash buhonero
RUN echo 'buhonero:gpc-test' | chpasswd
USER root 
COPY --from=builder /app/app /usr/local/bin/app
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/app /usr/local/bin/entrypoint.sh
USER buhonero
WORKDIR /home/buhonero
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
