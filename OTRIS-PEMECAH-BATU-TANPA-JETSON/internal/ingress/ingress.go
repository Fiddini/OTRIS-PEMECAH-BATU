package ingress

// PB-01 PROTOCOL INGRESS - Penjaga Pintu - RS232-FIRST READY
// Prinsip: PASSIVE FIRST, jangan ganggu alat
// Wakaf Ayahanda Wijoyo Kusumo - Allahu Latif 125x
// Spec: 2026-08-25-wijoyo-kusumo-v2-rs232-first-design.md

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/otris-id/otris-pemecah-batu/internal/model"
	"go.bug.st/serial"
)

type Config struct {
	Mode       string // passive, active, file, serial
	Transport  string // tcp, serial, file
	ListenAddr string // 0.0.0.0:4000
	TargetAddr string // untuk active mode
	FilePath   string // untuk file mode, atau fallback port path untuk serial
	SerialPort string // /dev/ttyUSB0, /dev/ttyS0, /dev/ttyACM0, /dev/cu.usbserial-xxx (Mac)
	BaudRate   int    // 9600, 19200, 115200 - default 9600 medis
}

type Ingress struct {
	cfg Config
}

func New(cfg Config) *Ingress {
	return &Ingress{cfg: cfg}
}

// Listen - hasilkan channel RawChunk, tidak pernah ubah byte
// RS232-FIRST: cek transport serial dulu
func (in *Ingress) Listen(ctx context.Context) (<-chan model.RawChunk, error) {
	out := make(chan model.RawChunk, 100)

	// RS232-FIRST CHECK - sesuai spec 2026-08-25
	if in.cfg.Transport == "serial" || in.cfg.Mode == "serial" {
		go in.listenSerial(ctx, out)
		return out, nil
	}

	switch in.cfg.Mode {
	case "file":
		go in.listenFile(ctx, out)
	case "passive":
		go in.listenPassive(ctx, out)
	case "active":
		go in.listenActive(ctx, out)
	default:
		return nil, fmt.Errorf("mode tidak dikenal: %s (gunakan passive|active|file|serial)", in.cfg.Mode)
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
				Bytes:      cp,
				ReceivedAt: time.Now(),
				Source:     fmt.Sprintf("file://%s", in.cfg.FilePath),
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

// PB-01 SERIAL ADAPTER - RS232 READY - 2 tahun tertunda, hari ini selesai
// Hukum: PASSIVE FIRST, jangan ganggu alat, jangan ubah 1 byte
func (in *Ingress) listenSerial(ctx context.Context, out chan<- model.RawChunk) {
	defer close(out)

	portPath := in.cfg.SerialPort
	if portPath == "" {
		portPath = in.cfg.FilePath // fallback: pakai --file untuk port path biar kompatibel CLI lama
	}
	if portPath == "" {
		portPath = "/dev/ttyUSB0"
	}

	baud := in.cfg.BaudRate
	if baud == 0 {
		baud = 9600 // default medis paling umum (Mindray, dll)
	}

	mode := &serial.Mode{
		BaudRate: baud,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	fmt.Printf("[PB-01] RS232-FIRST READY - Serial Adapter: %s @%d baud - PASSIVE FIRST\n", portPath, baud)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		fmt.Printf("[PB-01] SERIAL OPEN %s @%d - menunggu alat...\n", portPath, baud)
		p, err := serial.Open(portPath, mode)
		if err != nil {
			fmt.Printf("[PB-01] gagal open serial %s: %v - retry 2s\n", portPath, err)
			time.Sleep(2 * time.Second)
			continue
		}

		// Beri waktu alat stabil
		time.Sleep(100 * time.Millisecond)

		buf := make([]byte, 8192)
		for {
			select {
			case <-ctx.Done():
				p.Close()
				fmt.Printf("[PB-01] serial closed by context\n")
				return
			default:
			}

			// Timeout 500ms biar tidak blocking forever, sesuai PB-04 buffer logic
			p.SetReadTimeout(500 * time.Millisecond)
			n, err := p.Read(buf)
			if n > 0 {
				// CLONE bytes - jangan ubah 1 byte, sesuai arsitektur v1.0
				cp := make([]byte, n)
				copy(cp, buf[:n])
				out <- model.RawChunk{
					Bytes:      cp,
					ReceivedAt: time.Now(), // akan di-enrich jadi WallUTC + MonotonicNS di PB-03 Timestamp
					Source:     fmt.Sprintf("serial://%s@%d", portPath, baud),
				}
			}
			if err != nil {
				// Timeout adalah normal untuk RS232, bukan error fatal
				continue
			}
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
				Bytes:      cp,
				ReceivedAt: time.Now(),
				Source:     fmt.Sprintf("tcp://%s", conn.RemoteAddr()),
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
