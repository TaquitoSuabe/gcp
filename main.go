package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

type PortConfig struct {
	ListenPort  int
	TargetPort  int
	SkipPackets uint32 
}

type ProxyStats struct {
	Connections    int
	BytesForwarded int64
	sync.Mutex
}

func processSocket(ctx context.Context, conn net.Conn, target string, skipPackets uint32, stats *ProxyStats) {
	defer conn.Close()
	stats.Lock()
	stats.Connections++
	stats.Unlock()

	defer func() {
		stats.Lock()
		stats.Connections--
		stats.Unlock()
	}()

	// Conectar al servidor remoto
	remoteConn, err := net.DialTimeout("tcp", target, 2*time.Second) // Reducir el timeout
	if err != nil {
		slog.Error("Connection failed", "target", target, "error", err)
		return
	}
	defer remoteConn.Close()

	// Enviar respuesta inicial
	if _, err := conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nContent-Length: 1048576000000\r\n\r\n")); err != nil {
		slog.Error("Failed to send upgrade response", "error", err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Copiar datos del cliente al servidor
	go func() {
		defer wg.Done()
		packetCount := uint32(0)
		buf := make([]byte, 4*1024) 

		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, err := conn.Read(buf)
				if err != nil {
					if err != io.EOF && !isClosedConnError(err) {
						slog.Warn("Client read error", "error", err)
					}
					return
				}

				if packetCount < skipPackets {
					packetCount++
					slog.Info("Skipping packet",
						"current", packetCount,
						"total", skipPackets,
						"client", conn.RemoteAddr())
					continue
				}

				if _, err := remoteConn.Write(buf[:n]); err != nil {
					if !isClosedConnError(err) {
						slog.Warn("Target write error", "error", err)
					}
					return
				}

				stats.Lock()
				stats.BytesForwarded += int64(n)
				stats.Unlock()
			}
		}
	}()

	// Copiar datos del servidor al cliente
	go func() {
		defer wg.Done()
		buf := make([]byte, 12*1024)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, err := remoteConn.Read(buf)
				if err != nil {
					if err != io.EOF && !isClosedConnError(err) {
						slog.Warn("Target read error", "error", err)
					}
					return
				}

				if _, err := conn.Write(buf[:n]); err != nil {
					if !isClosedConnError(err) {
						slog.Warn("Client write error", "error", err)
					}
					return
				}

				stats.Lock()
				stats.BytesForwarded += int64(n)
				stats.Unlock()
			}
		}
	}()

	wg.Wait()
	slog.Info("Connection closed", "client", conn.RemoteAddr(), "target", target)
}

func isClosedConnError(err error) bool {
	if err == io.EOF {
		return true
	}
	if opErr, ok := err.(*net.OpError); ok {
		return opErr.Err.Error() == "use of closed network connection"
	}
	return false
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func main() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(handler))

	var listenPorts, targetPorts []int
	var skipPackets []uint32

	flag.Func("p", "Puerto en el que el proxy escuchará (puede usarse múltiples veces)", func(s string) error {
		port, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("invalid port: %v", err)
		}
		listenPorts = append(listenPorts, port)
		return nil
	})

	flag.Func("l", "Puerto al que el proxy redirigirá (puede usarse múltiples veces)", func(s string) error {
		port, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("invalid port: %v", err)
		}
		targetPorts = append(targetPorts, port)
		return nil
	})

	flag.Func("s", "Número de paquetes a saltar (puede usarse múltiples veces)", func(s string) error {
		skip, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("invalid skip_packets: %v", err)
		}
		skipPackets = append(skipPackets, uint32(skip))
		return nil
	})

	flag.Parse()

	if len(listenPorts) == 0 || len(targetPorts) == 0 || len(listenPorts) != len(targetPorts) {
		slog.Error("Número inválido de argumentos. Debes proporcionar pares de -p y -l.")
		flag.Usage()
		os.Exit(1)
	}

	if len(skipPackets) == 0 {
		skipPackets = make([]uint32, len(listenPorts))
	} else if len(skipPackets) != len(listenPorts) {
		slog.Error("Número inválido de argumentos -s. Debe coincidir con el número de -p y -l.")
		flag.Usage()
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signalCh
		slog.Info("\nReceived shutdown signal")
		cancel()
	}()

	stats := &ProxyStats{}

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stats.Lock()
				activeConnections := stats.Connections
				bytesForwarded := stats.BytesForwarded
				stats.Unlock()

				fmt.Print("\033[2K\r")
				fmt.Printf("Conexiones activas: %d     Transferencia: %s", activeConnections, formatBytes(bytesForwarded))
			case <-ctx.Done():
				return
			}
		}
	}()

	var wg sync.WaitGroup

	for i := 0; i < len(listenPorts); i++ {
		wg.Add(1)

		go func(pc PortConfig) {
			defer wg.Done()

			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", pc.ListenPort))
			if err != nil {
				slog.Error("Failed to start listener", "port", pc.ListenPort, "error", err)
				return
			}

			go func() {
				<-ctx.Done()
				listener.Close()
			}()

			slog.Info("Listener started",
				"port", pc.ListenPort,
				"target_port", pc.TargetPort,
				"skip_packets", pc.SkipPackets)

			for {
				select {
				case <-ctx.Done():
					return
				default:
					conn, err := listener.Accept()
					if err != nil {
						if !isClosedConnError(err) {
							slog.Warn("Accept error", "port", pc.ListenPort, "error", err)
						}
						continue 
					}

					slog.Info("New connection", "client", conn.RemoteAddr(), "port", pc.ListenPort)
					target := fmt.Sprintf("127.0.0.1:%d", pc.TargetPort)
					go processSocket(ctx, conn, target, pc.SkipPackets, stats)
				}
			}
		}(PortConfig{
			ListenPort:  listenPorts[i],
			TargetPort:  targetPorts[i],
			SkipPackets: skipPackets[i],
		})
	}

	wg.Wait()
	slog.Info("Proxy shutdown complete")
}
