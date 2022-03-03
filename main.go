package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"

	"golang.org/x/sync/errgroup"
	//"golang.org/x/crypto/ssh"
)

func read32(r io.Reader) (uint32, error) {
	var b [4]byte
	if n, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	} else if n != 4 {
		return 0, fmt.Errorf("%d != 4", n)
	}

	return binary.BigEndian.Uint32(b[:]), nil
}

func readPacket(r io.Reader) ([]byte, error) {
	size, err := read32(r)
	if err != nil {
		return nil, err
	}

	pkt := make([]byte, size - 4)
	if n, err := io.ReadFull(r, pkt); err != nil {
		return nil, err
	} else if n != int(size - 4) {
		return nil, fmt.Errorf("%d != 4", size - 4)
	}
	return pkt, nil
}

func write32(w io.Writer, v uint32) error {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)

	if n, err := w.Write(b[:]); err != nil {
		return err
	} else if n != 4 {
		return fmt.Errorf("%d != 4", n)
	}

	return nil
}

func writePacket(w io.Writer, pkt []byte) error {
	if err := write32(w, uint32(len(pkt) + 4)); err != nil {
		return err
	}

	if n, err := w.Write(pkt); err != nil {
		return err
	} else if n != len(pkt) {
		return fmt.Errorf("%d != n", len(pkt))
	}

	return nil
}

func serve(cx context.Context, conn net.Conn) error {
	for {
		pkt, err := readPacket(conn)
		if err != nil {
			return err
		}

		if len(pkt) < 4 {
			return fmt.Errorf("Unexpected format.")
		}

		if binary.BigEndian.Uint32(pkt[:4]) == 80877103 {
			// SSLRequest
			if n, err := conn.Write([]byte("N")); err != nil {
				return err
			} else if n != 1 {
				return fmt.Errorf("%d != 1", n)
			}
			continue
		}

		// StartupMessage
		major := binary.BigEndian.Uint16(pkt[0:2])
		minor := binary.BigEndian.Uint16(pkt[2:4])
		if major != 3 || minor != 0 {
			return fmt.Errorf("Unsupported version.")
		}

		kv := bytes.Split(pkt[4:], []byte { 0 })
		if len(kv) % 2 != 0 {
			return fmt.Errorf("Unexpected format.")
		}
		for i := 0; i < len(kv) / 2; i ++ {
			// decide upstream
			k := kv[(i * 2) + 0]
			v := kv[(i * 2) + 1]
			fmt.Printf("%s %s\n", k, v)
		}

		up, err := net.Dial("tcp", "[::1]:5432")
		if err != nil {
			return err
		}
		defer up.Close()

		if err := writePacket(up, pkt); err != nil {
			return nil
		}

		eg, _ := errgroup.WithContext(cx)
		eg.Go(func() error {
			_, err = io.Copy(up, conn)
			return err
		})
		eg.Go(func() error {
			_, err = io.Copy(conn, up)
			return err
		})
		return eg.Wait()
	}
}

func main() {
	l, err := net.Listen("tcp", "[::1]:15432")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(-1)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		go func() {
			defer conn.Close()
			if err := serve(context.TODO(), conn); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}()
	}
}
