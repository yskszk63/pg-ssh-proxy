package main

import (
	"bytes"
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
