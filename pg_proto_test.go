package main

import (
	"bytes"
	"fmt"
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

func TestRawInitialPacketRead(t *testing.T) {
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

func TestRawInitialPacketWrite(t *testing.T) {
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

func TestRawInitialPacketWriteErr1(t *testing.T) {
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

func TestRawInitialPacketWriteErr2(t *testing.T) {
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

func TestSerde(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			"startupMessage",
			[]byte{
				0x00, 0x00, 0x00, 0x54, 0x00, 0x03, 0x00, 0x00, 0x61, 0x70, 0x70, 0x6c, 0x69, 0x63, 0x61, 0x74,
				0x69, 0x6f, 0x6e, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x00, 0x70, 0x73, 0x71, 0x6c, 0x00, 0x63, 0x6c,
				0x69, 0x65, 0x6e, 0x74, 0x5f, 0x65, 0x6e, 0x63, 0x6f, 0x64, 0x69, 0x6e, 0x67, 0x00, 0x55, 0x54,
				0x46, 0x38, 0x00, 0x64, 0x61, 0x74, 0x61, 0x62, 0x61, 0x73, 0x65, 0x00, 0x70, 0x6f, 0x73, 0x74,
				0x67, 0x72, 0x65, 0x73, 0x00, 0x75, 0x73, 0x65, 0x72, 0x00, 0x70, 0x6f, 0x73, 0x74, 0x67, 0x72,
				0x65, 0x73, 0x00, 0x00,
			},
		},
		{
			"sslRequest",
			[]byte{
				0x00, 0x00, 0x00, 0x08, 0x04, 0xd2, 0x16, 0x2f,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var pkt rawInitialPacket
			if err := pkt.read(bytes.NewBuffer(test.data)); err != nil {
				t.Fatal(err)
			}
			conc, err := pkt.toConcrete()
			if err != nil {
				t.Fatal(err)
			}
			pkt2 := conc.toRaw()
			b := &bytes.Buffer{}
			if err := pkt2.write(b); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(b.Bytes(), test.data) {
				t.Fatalf("%#v != %#v", b.Bytes(), test.data)
			}
		})
	}
}

func TestToConcreteErr(t *testing.T) {
	tests := []struct {
		name string
		data rawInitialPacket
		err  string
	}{
		{
			name: "unknown",
			data: rawInitialPacket{0xFF, 0xFF, 0xFF, 0xFF},
			err:  "unknown packet.",
		},
		{
			name: "empty",
			data: rawInitialPacket{},
			err:  "unexpected EOF",
		},
		{
			name: "eof1",
			data: rawInitialPacket{0x00, 0x03, 0x00, 0x00, 0x01},
			err:  "unexpected EOF",
		},
		{
			name: "eof1",
			data: rawInitialPacket{0x00, 0x03, 0x00, 0x00, 0x01, 0x00, 0x01},
			err:  "unexpected EOF",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.data.toConcrete(); err != nil {
				if err.Error() != test.err {
					t.Fatal(err)
				}
			} else {
				t.Fail()
			}
		})
	}
}

func TestStartupMessageDatabase(t *testing.T) {
	if (&startupMessage{map[string]string{}}).database() != nil {
		t.Fail()
	}
	if *(&startupMessage{map[string]string{"database": "ok"}}).database() != "ok" {
		t.Fail()
	}
}

func TestStartupMessageSetDatabase(t *testing.T) {
	m := &startupMessage{map[string]string{}}
	m.setDataabse("db")
	if *(m.database()) != "db" {
		t.Fail()
	}
}

func TestMust(t *testing.T) {
	defer func() {
		err := recover()
		if err.(error).Error() != "err" {
			t.Fatal(err)
		}
	}()
	must(fmt.Errorf("err"))
}

func TestRawPacketWrite(t *testing.T) {
	tests := []struct {
		name  string
		data  rawPacket
		wants []byte
	}{
		{
			name:  "empty",
			data:  rawPacket{0xFF, []byte{}},
			wants: []byte{0xFF, 0x00, 0x00, 0x00, 0x04},
		},
		{
			name:  "size1",
			data:  rawPacket{0xFF, []byte{0xFF}},
			wants: []byte{0xFF, 0x00, 0x00, 0x00, 0x05, 0xFF},
		},
		{
			name:  "size1",
			data:  rawPacket{0xFF, []byte{0xFF, 0xEE}},
			wants: []byte{0xFF, 0x00, 0x00, 0x00, 0x06, 0xFF, 0xEE},
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

func TestRawlPacketWriteErr1(t *testing.T) {
	r, w := io.Pipe()
	go func() {
		defer r.Close()

		if _, err := io.ReadFull(r, make([]byte, 4)); err != nil {
			panic(err)
		}
	}()
	p := rawPacket{0xFF, []byte{}}
	if err := p.write(w); err != nil {
		if err.Error() != "io: read/write on closed pipe" {
			t.Fatal(err)
		}
		return
	}
	t.Fail()
}

func TestRawlPacketWriteErr2(t *testing.T) {
	r, w := io.Pipe()
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}

	p := rawPacket{0xFF, []byte{0xFF}}
	if err := p.write(w); err != nil {
		if err.Error() != "io: read/write on closed pipe" {
			t.Fatal(err)
		}
		return
	}
	t.Fail()
}

func TestRawlPacketWriteErr3(t *testing.T) {
	r, w := io.Pipe()
	go func() {
		defer r.Close()

		if _, err := io.ReadFull(r, make([]byte, 5)); err != nil {
			panic(err)
		}
	}()
	p := rawPacket{0xFF, []byte{}}
	if err := p.write(w); err != nil {
		if err.Error() != "io: read/write on closed pipe" {
			t.Fatal(err)
		}
		return
	}
	t.Fail()
}

func TestPacketToRaw(t *testing.T) {
	tests := []struct {
		name  string
		data  packet
		wants rawPacket
	}{
		{
			"errorResponse",
			&errorResponse{
				fields: []errorResponseField{
					{
						code:  'M',
						value: "OK",
					},
				},
			},
			rawPacket{'E', []byte{0x4d, 0x4f, 0x4b, 0x00, 0x00}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := test.data.toRaw()
			if !reflect.DeepEqual(raw, test.wants) {
				t.Fatalf("%#v != %#v", raw, test.wants)
			}
		})
	}
}
