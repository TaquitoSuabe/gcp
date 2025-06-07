#!/bin/sh
/usr/sbin/sshd -D &
echo "Iniciando proxy en el puerto $PORT, redirigiendo a 127.0.0.1:22"
/usr/local/bin/app -p "$PORT" -l 22 -s 0
