#!/bin/sh

export DHOST=${DHOST:-"127.0.0.1"}
export DPORT=${DPORT:-"40000"}
export PORT=${PORT:-"8080"}
export PACKSKIP=${PACKSKIP:-"1"}
echo "Iniciando Dropbear en segundo plano en puerto $DPORT..."
tmux new-session -d -s ssh_session "/usr/sbin/dropbear -R -E -F -p $DPORT"
echo "Iniciando proxy en primer plano..."
exec /usr/local/bin/app
