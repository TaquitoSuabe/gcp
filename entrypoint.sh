#!/bin/sh
echo "Iniciando sesión de tmux para Dropbear en puerto 22..."
tmux new-session -d -s ssh_session '/usr/sbin/dropbear -R -E -F -p 22'
sleep 2
echo "Iniciando proxy en primer plano..."
/usr/local/bin/app -p "$PORT" -l 22 -s 1
