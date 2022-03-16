package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"
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
			if errors.Is(err, io.EOF) {
				return "", io.ErrUnexpectedEOF
			}
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

type initialPacket interface {
	toRaw() rawInitialPacket
}

func (p *rawInitialPacket) toConcrete() (initialPacket, error) {
	b := bytes.NewBuffer(*p)
	v, err := read32(b)
	if err != nil {
		if errors.Is(io.EOF, err) {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, err
	}

	switch v {
	case 196608:
		p := make(map[string]string)
		for {
			k, err := readString(b)
			if err != nil {
				return nil, err
			}
			if k == "" {
				return &startupMessage{p}, nil
			}

			v, err := readString(b)
			if err != nil {
				return nil, err
			}
			p[k] = v
		}

	case 80877103:
		return &sslRequest{}, nil

	default:
		return nil, fmt.Errorf("unknown packet.")
	}
}

func must(err error) {
	if err != nil {
		panic(err)
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

func (v *startupMessage) setDataabse(name string) {
	v.params["database"] = name
}

func (v *startupMessage) toRaw() rawInitialPacket {
	b := &bytes.Buffer{}

	must(write32(b, 196608))

	keys := make([]string, 0, len(v.params))
	for k := range v.params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		must(writeString(b, k))
		must(writeString(b, v.params[k]))
	}
	must(b.WriteByte(0))
	return rawInitialPacket(b.Bytes())
}

type sslRequest struct{}

func (v *sslRequest) toRaw() rawInitialPacket {
	b := &bytes.Buffer{}

	must(write32(b, 80877103))
	return rawInitialPacket(b.Bytes())
}

type rawPacket struct {
	header byte
	data   []byte
}

func (p *rawPacket) write(w io.Writer) error {
	if _, err := w.Write([]byte{p.header}); err != nil {
		return err
	}
	if err := write32(w, uint32(len(p.data)+4)); err != nil {
		return err
	}
	if _, err := w.Write(p.data); err != nil {
		return err
	}
	return nil
}

type packet interface {
	toRaw() rawPacket
}

type errorResponseField struct {
	code  byte
	value string
}

type errorResponse struct {
	fields []errorResponseField
}

func (v *errorResponse) toRaw() rawPacket {
	b := &bytes.Buffer{}

	for _, f := range v.fields {
		must(b.WriteByte(f.code))
		must(writeString(b, f.value))
	}
	must(b.WriteByte(0))

	return rawPacket{
		header: 'E',
		data:   b.Bytes(),
	}
}
