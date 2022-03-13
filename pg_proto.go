package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

func read32(r io.Reader) (uint32, error) {
	var b [4]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint32(b[:]), nil
}

func write32(w io.Writer, v uint32) error {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)

	if _, err := w.Write(b[:]); err != nil {
		return err
	}

	return nil
}

func readString(r io.Reader) (string, error) {
	v := make([]byte, 0)
	for {
		b := make([]byte, 1)
		if _, err := io.ReadFull(r, b); err != nil {
			return "", err
		}
		if b[0] == 0 {
			return string(v), nil
		}
		v = append(v, b...)
	}
}

func writeString(w io.Writer, v string) error {
	if _, err := w.Write([]byte(v)); err != nil {
		return err
	}
	if _, err := w.Write([]byte{0}); err != nil {
		return err
	}
	return nil
}

type rawInitialPacket []byte

func (p *rawInitialPacket) read(r io.Reader) error {
	size, err := read32(r)
	if err != nil {
		return err
	}
	if size < 4 {
		return fmt.Errorf("invalid packet size")
	}

	pkt := make([]byte, size-4)
	if _, err := io.ReadFull(r, pkt); err != nil {
		if errors.Is(io.EOF, err) {
			return io.ErrUnexpectedEOF
		}
		return err
	}
	*p = pkt
	return nil
}

func (p *rawInitialPacket) write(w io.Writer) error {
	if err := write32(w, uint32(len(*p)+4)); err != nil {
		return err
	}

	if _, err := w.Write(*p); err != nil {
		return err
	}
	return nil
}

func (p *rawInitialPacket) toConcrete() (interface{}, error) {
	b := bytes.NewBuffer(*p)
	v, err := read32(b)
	if err != nil {
		return nil, err
	}

	switch v {
	case 196608:
		p := make(map[string]string)
		for {
			k, err := readString(b)
			if err != nil && errors.Is(io.EOF, err) {
				return nil, err
			}
			if k == "" {
				return startupMessage{p}, nil
			}

			v, err := readString(b)
			if err != nil {
				return nil, err
			}
			p[k] = v
		}

	case 80877103:
		return sslRequest{}, nil

	default:
		return nil, fmt.Errorf("unknown packet.")
	}
}

type startupMessage struct {
	params map[string]string
}

func (v *startupMessage) database() *string {
	if val, exists := v.params["database"]; exists {
		return &val
	}
	return nil
}

func (v *startupMessage) toRaw() rawInitialPacket {
	b := &bytes.Buffer{}

	if err := write32(b, 196608); err != nil {
		panic(err)
	}

	for k, v := range v.params {
		if err := writeString(b, k); err != nil {
			panic(err)
		}
		if err := writeString(b, v); err != nil {
			panic(err)
		}
	}
	if _, err := b.Write([]byte{0}); err != nil {
		panic(err)
	}
	return rawInitialPacket(b.Bytes())
}

type sslRequest struct{}
