#!/bin/sh
echo "Iniciando proxy en segundo plano..."
/usr/local/bin/app -p "$PORT" -l 22 -s 2 &
echo "Iniciando servidor SSH Dropbear en puerto 22..."
/usr/sbin/dropbear -R -E -F -p 22
