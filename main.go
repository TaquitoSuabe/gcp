package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

func generateWebSocketAccept() string {
	bytes := make([]byte, 20)
	rand.Read(bytes)
	return base64.StdEncoding.EncodeToString(bytes)
}

func handleConnection(clientConn net.Conn, dhost string, dport int, packetsToSkip int) {
	defer clientConn.Close()
	
	// Enviar respuesta HTTP 101 (WebSocket handshake simulado)
	response := fmt.Sprintf(
		"HTTP/1.1 101 Switching Protocols\r\n"+
		"Connection: Upgrade\r\n"+
		"Date: %s\r\n"+
		"Sec-WebSocket-Accept: %s\r\n"+
		"Upgrade: websocket\r\n"+
		"Server: go-proxy/1.0\r\n\r\n",
		time.Now().UTC().Format(time.RFC1123),
		generateWebSocketAccept(),
	)
	
	if _, err := clientConn.Write([]byte(response)); err != nil {
		log.Printf("[ERROR] Failed to send handshake: %v", err)
		return
	}
	
	// Conectar al servidor de destino (Dropbear)
	targetConn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", dhost, dport), 5*time.Second)
	if err != nil {
		log.Printf("[ERROR] Failed to connect to target: %v", err)
		return
	}
	defer targetConn.Close()
	
	log.Printf("[INFO] Connection established: %s -> %s:%d", 
		clientConn.RemoteAddr(), dhost, dport)
	
	// Canal para coordinar el cierre
	done := make(chan bool, 2)
	
	// Goroutine para datos cliente -> servidor (con skip de paquetes)
	go func() {
		defer func() { done <- true }()
		
		packetCount := 0
		buffer := make([]byte, 4096)
		
		for {
			n, err := clientConn.Read(buffer)
			if err != nil {
				if err != io.EOF {
					log.Printf("[CLIENT] Read error: %v", err)
				}
				return
			}
			
			// Lógica de skip de paquetes (igual que el JS)
			if packetCount < packetsToSkip {
				packetCount++
				continue
			} else if packetCount == packetsToSkip {
				if _, err := targetConn.Write(buffer[:n]); err != nil {
					log.Printf("[TARGET] Write error: %v", err)
					return
				}
			}
			
			if packetCount > packetsToSkip {
				packetCount = packetsToSkip
			}
		}
	}()
	
	// Goroutine para datos servidor -> cliente
	go func() {
		defer func() { done <- true }()
		
		if _, err := io.Copy(clientConn, targetConn); err != nil {
			if err != io.EOF {
				log.Printf("[TARGET] Copy error: %v", err)
			}
		}
	}()
	
	// Esperar a que termine cualquiera de las dos goroutines
	<-done
	log.Printf("[INFO] Connection closed: %s", clientConn.RemoteAddr())
}

func main() {
	// Variables de entorno (igual que el JS)
	dhost := getEnv("DHOST", "127.0.0.1")
	dport, _ := strconv.Atoi(getEnv("DPORT", "40000"))
	mainPort := getEnv("PORT", "8080")
	packetsToSkip, _ := strconv.Atoi(getEnv("PACKSKIP", "1"))
	
	// Crear servidor TCP
	listener, err := net.Listen("tcp", ":"+mainPort)
	if err != nil {
		log.Fatalf("[FATAL] Failed to start server: %v", err)
	}
	defer listener.Close()
	
	log.Printf("[INFO] Server started on port: %s", mainPort)
	log.Printf("[INFO] Redirecting requests to: %s at port %d", dhost, dport)
	log.Printf("[INFO] Packets to skip: %d", packetsToSkip)
	
	// Aceptar conexiones
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[ERROR] Accept failed: %v", err)
			continue
		}
		
		// Manejar cada conexión en una goroutine separada
		go handleConnection(conn, dhost, dport, packetsToSkip)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
