package main

import (
	"encoding/binary"
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

type rawPacket []byte

func (p *rawPacket) read(r io.Reader) error {
	size, err := read32(r)
	if err != nil {
		return err
	}

	pkt := make([]byte, size-4)
	if _, err := io.ReadFull(r, pkt); err != nil {
		return err
	}
	*p = pkt
	return nil
}

func (p *rawPacket) write(w io.Writer) error {
	if err := write32(w, uint32(len(*p)+4)); err != nil {
		return err
	}

	if _, err := w.Write(*p); err != nil {
		return err
	}
	return nil
}
