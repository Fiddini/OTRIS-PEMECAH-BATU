package ingress

// PB-01 PROTOCOL INGRESS - Penjaga Pintu
// Prinsip: PASSIVE FIRST, jangan ganggu alat

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/otris-id/otris-pemecah-batu/internal/model"
)

type Config struct {
	Mode      string // passive, active, file
	Transport string // tcp, serial, file
	ListenAddr string // 0.0.0.0:4000
	TargetAddr string // untuk active mode
	FilePath   string // untuk file mode
}

type Ingress struct {
	cfg Config
}

func New(cfg Config) *Ingress {
	return &Ingress{cfg: cfg}
}

// Listen - hasilkan channel RawChunk, tidak pernah ubah byte
func (in *Ingress) Listen(ctx context.Context) (<-chan model.RawChunk, error) {
	out := make(chan model.RawChunk, 100)

	switch in.cfg.Mode {
	case "file":
		go in.listenFile(ctx, out)
	case "passive":
		go in.listenPassive(ctx, out)
	case "active":
		go in.listenActive(ctx, out)
	default:
		return nil, fmt.Errorf("mode tidak dikenal: %s", in.cfg.Mode)
	}

	return out, nil
}

func (in *Ingress) listenPassive(ctx context.Context, out chan<- model.RawChunk) {
	defer close(out)
	ln, err := net.Listen("tcp", in.cfg.ListenAddr)
	if err != nil {
		fmt.Printf("[PB-01] gagal listen %s: %v\n", in.cfg.ListenAddr, err)
		return
	}
	fmt.Printf("[PB-01] PASSIVE LISTEN di %s - menunggu alat connect...\n", in.cfg.ListenAddr)
	
	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}
		fmt.Printf("[PB-01] Alat connect dari %s\n", conn.RemoteAddr())
		go in.handleConn(ctx, conn, out)
	}
}

func (in *Ingress) listenActive(ctx context.Context, out chan<- model.RawChunk) {
	defer close(out)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		fmt.Printf("[PB-01] ACTIVE CONNECT ke %s\n", in.cfg.TargetAddr)
		conn, err := net.Dial("tcp", in.cfg.TargetAddr)
		if err != nil {
			fmt.Printf("[PB-01] gagal connect, retry 2s: %v\n", err)
			time.Sleep(2 * time.Second)
			continue
		}
		in.handleConn(ctx, conn, out)
		time.Sleep(1 * time.Second)
	}
}

func (in *Ingress) listenFile(ctx context.Context, out chan<- model.RawChunk) {
	defer close(out)
	f, err := os.Open(in.cfg.FilePath)
	if err != nil {
		fmt.Printf("[PB-01] gagal buka file %s: %v\n", in.cfg.FilePath, err)
		return
	}
	defer f.Close()
	buf := make([]byte, 8192)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		n, err := f.Read(buf)
		if n > 0 {
			cp := make([]byte, n)
			copy(cp, buf[:n])
			out <- model.RawChunk{
				Bytes: cp,
				ReceivedAt: time.Now(),
				Source: fmt.Sprintf("file://%s", in.cfg.FilePath),
			}
		}
		if err == io.EOF {
			return
		}
		if err != nil {
			fmt.Printf("[PB-01] read error: %v\n", err)
			return
		}
	}
}

func (in *Ingress) handleConn(ctx context.Context, conn net.Conn, out chan<- model.RawChunk) {
	defer conn.Close()
	buf := make([]byte, 64*1024)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := conn.Read(buf)
		if n > 0 {
			cp := make([]byte, n)
			copy(cp, buf[:n])
			out <- model.RawChunk{
				Bytes: cp,
				ReceivedAt: time.Now(),
				Source: fmt.Sprintf("tcp://%s", conn.RemoteAddr()),
			}
		}
		if err != nil {
			if err != io.EOF {
				// fmt.Printf("[PB-01] conn read err: %v\n", err)
			}
			return
		}
	}
}
