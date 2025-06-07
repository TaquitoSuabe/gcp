#!/bin/bash

echo "=== Iniciando Proxy SSH para Cloud Run ==="

export DHOST=${DHOST:-"127.0.0.1"}
export DPORT=${DPORT:-"40000"}
export PORT=${PORT:-"8080"}
export PACKSKIP=${PACKSKIP:-"1"}

echo "[INFO] Verificando claves SSH de Dropbear..."
if [ ! -f /etc/dropbear/dropbear_rsa_host_key ]; then
    echo "[INFO] Generando clave RSA para Dropbear..."
    dropbearkey -t rsa -f /etc/dropbear/dropbear_rsa_host_key
fi

if [ ! -f /etc/dropbear/dropbear_ecdsa_host_key ]; then
    echo "[INFO] Generando clave ECDSA para Dropbear..."
    dropbearkey -t ecdsa -f /etc/dropbear/dropbear_ecdsa_host_key
fi

echo "[INFO] Iniciando Dropbear SSH en puerto $DPORT..."
tmux new-session -d -s ssh_session "dropbear -REF -p $DPORT -W 65535"

sleep 3

if ! netstat -tuln | grep -q ":$DPORT "; then
    echo "[ERROR] Dropbear no se inició correctamente en puerto $DPORT"
    echo "[INFO] Intentando iniciar Dropbear directamente..."
    dropbear -REF -p $DPORT -W 65535 &
    sleep 2
fi

if netstat -tuln | grep -q ":$DPORT "; then
    echo "[INFO] Dropbear SSH corriendo en puerto $DPORT"
else
    echo "[ERROR] No se pudo iniciar Dropbear SSH"
    exit 1
fi

echo "[INFO] Configuración:"
echo "  - Proxy puerto: $PORT"
echo "  - SSH puerto: $DPORT"
echo "  - Host destino: $DHOST"
echo "  - Paquetes a saltar: $PACKSKIP"

echo "[INFO] Iniciando proxy en primer plano..."

exec /usr/local/bin/proxy
