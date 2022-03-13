package main

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func TestRead32(t *testing.T) {
	tests := []struct {
		name  string
		data  []byte
		wants uint32
		err   string
	}{
		{
			name:  "0",
			data:  []byte{0, 0, 0, 0},
			wants: 0,
		},
		{
			name:  "1",
			data:  []byte{0, 0, 0, 1},
			wants: 1,
		},
		{
			name:  "0xFFFF_FFFF",
			data:  []byte{0xFF, 0xFF, 0xFF, 0xFF},
			wants: 0xFFFF_FFFF,
		},
		{
			name: "EMPTY",
			data: []byte{},
			err:  "EOF",
		},
		{
			name: "few",
			data: []byte{0x00},
			err:  "unexpected EOF",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			v, err := read32(bytes.NewBuffer(test.data))
			if test.err != "" {
				if err == nil {
					t.Fatal("no error occurred.")
				}
				if err.Error() != test.err {
					t.Fatalf("%s != %s", err, test.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			if v != test.wants {
				t.Fatalf("%d != %d", v, test.wants)
			}
		})
	}
}

func TestWrite32(t *testing.T) {
	tests := []struct {
		name  string
		val   uint32
		wants []byte
		err   string
	}{
		{
			name:  "0",
			val:   0,
			wants: []byte{0, 0, 0, 0},
		},
		{
			name:  "1",
			val:   1,
			wants: []byte{0, 0, 0, 1},
		},
		{
			name:  "0xFFFF_FFFF",
			val:   0xFFFF_FFFF,
			wants: []byte{0xFF, 0xFF, 0xFF, 0xFF},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b := bytes.NewBuffer([]byte{})
			if err := write32(b, test.val); err != nil {
				if test.err != "" && test.err != err.Error() {
					t.Fatal(err)
				}
				return
			} else if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(b.Bytes(), test.wants) {
				t.Fatalf("%#v != %#v", b.Bytes(), test.wants)
			}
		})
	}
}

func TestWrite32Err(t *testing.T) {
	r, w := io.Pipe()
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}

	if err := write32(w, 0); err != nil {
		if err.Error() != "io: read/write on closed pipe" {
			t.Fatal(err)
		}
	} else {
		t.Fail()
	}
}

func TestReadString(t *testing.T) {
	tests := []struct {
		name  string
		data  []byte
		wants string
		err   string
	}{
		{
			name:  "empty",
			data:  []byte{0},
			wants: "",
		},
		{
			name:  "size 1",
			data:  []byte{'a', 0},
			wants: "a",
		},
		{
			name: "eof",
			data: []byte{'a'},
			err:  "unexpected EOF",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b := bytes.NewBuffer(test.data)
			v, err := readString(b)
			if test.err != "" {
				if err == nil {
					t.Fatal()
				}
				if err.Error() != test.err {
					t.Fatal(err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			if v != test.wants {
				t.Fatalf("%s != %s", v, test.wants)
			}
		})
	}
}

func TestWriteString(t *testing.T) {
	tests := []struct {
		name  string
		data  string
		wants []byte
	}{
		{
			name:  "empty",
			data:  "",
			wants: []byte{0},
		},
		{
			name:  "size 1",
			data:  "a",
			wants: []byte{'a', 0},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b := &bytes.Buffer{}
			if err := writeString(b, test.data); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(b.Bytes(), test.wants) {
				t.Fatalf("%s != %s", b.Bytes(), test.wants)
			}
		})
	}
}

func TestWriteStringErr(t *testing.T) {
	r, w := io.Pipe()
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}

	if err := writeString(w, ""); err == nil {
		t.Fail()
	} else if err.Error() != "io: read/write on closed pipe" {
		t.Fatal(err)
	}
}

func TestWriteStringErr2(t *testing.T) {
	r, w := io.Pipe()
	go func() {
		defer r.Close()
		io.ReadFull(r, make([]byte, 1))
	}()

	if err := writeString(w, "a"); err == nil {
		t.Fail()
	} else if err.Error() != "io: read/write on closed pipe" {
		t.Fatal(err)
	}
}

func TestRawPacketRead(t *testing.T) {
	tests := []struct {
		name  string
		data  []byte
		wants rawInitialPacket
		err   string
	}{
		{
			name:  "test empty",
			data:  []byte{0x00, 0x00, 0x00, 0x04},
			wants: rawInitialPacket{},
		},
		{
			name:  "test size1",
			data:  []byte{0x00, 0x00, 0x00, 0x05, 0xFF},
			wants: rawInitialPacket{0xFF},
		},
		{
			name:  "test size2",
			data:  []byte{0x00, 0x00, 0x00, 0x06, 0xFF, 0xEE},
			wants: rawInitialPacket{0xFF, 0xEE},
		},
		{
			name: "test EOF 1",
			data: []byte{},
			err:  "EOF",
		},
		{
			name: "test EOF 2",
			data: []byte{0x00},
			err:  "unexpected EOF",
		},
		{
			name: "test EOF 3",
			data: []byte{0x00, 0x00},
			err:  "unexpected EOF",
		},
		{
			name: "test EOF 4",
			data: []byte{0x00, 0x00, 0x00},
			err:  "unexpected EOF",
		},
		{
			name: "test EOF 5",
			data: []byte{0x00, 0x00, 0x00, 0x05},
			err:  "unexpected EOF",
		},
		{
			name: "test invalid packet size 1",
			data: []byte{0x00, 0x00, 0x00, 0x00},
			err:  "invalid packet size",
		},
		{
			name: "test invalid packet size 2",
			data: []byte{0x00, 0x00, 0x00, 0x01},
			err:  "invalid packet size",
		},
		{
			name: "test invalid packet size 3",
			data: []byte{0x00, 0x00, 0x00, 0x02},
			err:  "invalid packet size",
		},
		{
			name: "test invalid packet size 4",
			data: []byte{0x00, 0x00, 0x00, 0x03},
			err:  "invalid packet size",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var pkt rawInitialPacket
			if err := pkt.read(bytes.NewBuffer(test.data)); err != nil {
				if test.err == "" || test.err != err.Error() {
					t.Fatal(err)
				}
				return
			}
			if test.err != "" {
				t.Fail()
			}
			if !reflect.DeepEqual(pkt, test.wants) {
				t.Fatalf("%#v != %#v", pkt, test.wants)
			}
		})
	}
}

func TestRawPacketWrite(t *testing.T) {
	tests := []struct {
		name  string
		data  rawInitialPacket
		wants []byte
	}{
		{
			name:  "empty",
			data:  rawInitialPacket{},
			wants: []byte{0x00, 0x00, 0x00, 0x04},
		},
		{
			name:  "size1",
			data:  rawInitialPacket{0xFF},
			wants: []byte{0x00, 0x00, 0x00, 0x05, 0xFF},
		},
		{
			name:  "size1",
			data:  rawInitialPacket{0xFF, 0xEE},
			wants: []byte{0x00, 0x00, 0x00, 0x06, 0xFF, 0xEE},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b := bytes.Buffer{}
			if err := test.data.write(&b); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(b.Bytes(), test.wants) {
				t.Fatalf("%#v != %#v", b.Bytes(), test.wants)
			}
		})
	}
}

func TestRawPacketWriteErr1(t *testing.T) {
	r, w := io.Pipe()
	go func() {
		defer r.Close()

		if _, err := io.ReadFull(r, make([]byte, 4)); err != nil {
			panic(err)
		}
	}()
	p := rawInitialPacket{0xFF}
	if err := p.write(w); err != nil {
		if err.Error() != "io: read/write on closed pipe" {
			t.Fatal(err)
		}
		return
	}
	t.Fail()
}

func TestRawPacketWriteErr2(t *testing.T) {
	r, w := io.Pipe()
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}

	p := rawInitialPacket{0xFF}
	if err := p.write(w); err != nil {
		if err.Error() != "io: read/write on closed pipe" {
			t.Fatal(err)
		}
		return
	}
	t.Fail()
}
