package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net"
	"reflect"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

const serverHostKey string = `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACCASYEiknzqmSCpmQ0x3SL9o2UzJwzr5CYZ2ZjWxWln3gAAAJAaWkH4GlpB
+AAAAAtzc2gtZWQyNTUxOQAAACCASYEiknzqmSCpmQ0x3SL9o2UzJwzr5CYZ2ZjWxWln3g
AAAEBsxeEBBdqO1L9Tfz0odYwxGr8YGoz5k7Ta8KnpJgvlP4BJgSKSfOqZIKmZDTHdIv2j
ZTMnDOvkJhnZmNbFaWfeAAAADXlza3N6azYzQGEyODU=
-----END OPENSSH PRIVATE KEY-----`
const serverHostKeyPub string = `ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIBJgSKSfOqZIKmZDTHdIv2jZTMnDOvkJhnZmNbFaWfe`
const identityKey string = `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACBit5xk9FVcvIxZKt273YehYqq0poecxw/gEcMV8zLmWQAAAJATvs2hE77N
oQAAAAtzc2gtZWQyNTUxOQAAACBit5xk9FVcvIxZKt273YehYqq0poecxw/gEcMV8zLmWQ
AAAECApp5/YbkiHHr3bGt8O41cey125xrESv5K5WIZlEuKzGK3nGT0VVy8jFkq3bvdh6Fi
qrSmh5zHD+ARwxXzMuZZAAAADXlza3N6azYzQGEyODU=
-----END OPENSSH PRIVATE KEY-----`
const identityKeyPub string = `ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGK3nGT0VVy8jFkq3bvdh6FiqrSmh5zHD+ARwxXzMuZZ`

type testDialSshTunnelFsFile struct {
	buf  *bytes.Buffer
	name string
}

func (v *testDialSshTunnelFsFile) Read(b []byte) (int, error) {
	return v.buf.Read(b)
}

func (v *testDialSshTunnelFsFile) Close() error {
	return nil
}

func (v *testDialSshTunnelFsFile) Stat() (fs.FileInfo, error) {
	return v, nil
}

func (v *testDialSshTunnelFsFile) Name() string {
	return v.name
}

func (v *testDialSshTunnelFsFile) Size() int64 {
	return int64(v.buf.Len())
}

func (*testDialSshTunnelFsFile) Mode() fs.FileMode {
	return 0o755
}

func (*testDialSshTunnelFsFile) ModTime() time.Time {
	return time.Time{}
}

func (*testDialSshTunnelFsFile) IsDir() bool {
	return false
}

func (*testDialSshTunnelFsFile) Sys() interface{} {
	return nil
}

type testDialSshTunnelFs struct {
	knownhosts string
}

func (v testDialSshTunnelFs) Open(fname string) (fs.File, error) {
	switch fname {
	case "/id_ed25519":
		return &testDialSshTunnelFsFile{
			buf:  bytes.NewBufferString(identityKey),
			name: fname,
		}, nil
	case "/known_hosts":
		return &testDialSshTunnelFsFile{
			buf:  bytes.NewBufferString(v.knownhosts),
			name: fname,
		}, nil
	}
	return nil, fs.ErrNotExist
}

func TestDialSshTunnel(t *testing.T) {
	l, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Fatal(err)
	}
	sconf := &ssh.ServerConfig{
		PublicKeyCallback: func(c ssh.ConnMetadata, pubkey ssh.PublicKey) (*ssh.Permissions, error) {
			return &ssh.Permissions{}, nil
		},
	}
	skey, err := ssh.ParsePrivateKey([]byte(serverHostKey))
	if err != nil {
		t.Fatal(err)
	}
	sconf.AddHostKey(skey)

	go func() {
		defer l.Close()

		nConn, err := l.Accept()
		if err != nil {
			panic(err)
		}
		defer nConn.Close()

		conn, chans, reqs, err := ssh.NewServerConn(nConn, sconf)
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		go ssh.DiscardRequests(reqs)
		for ch := range chans {
			switch ch.ChannelType() {
			case "direct-tcpip":
				ch, reqs, err := ch.Accept()
				if err != nil {
					panic(err)
				}
				defer ch.Close()
				go ssh.DiscardRequests(reqs)

				b := make([]byte, 2, 2)
				if _, err := io.ReadFull(ch, b); err != nil {
					panic(err)
				}
				ch.Write(b)
				return
			default:
				ch.Reject(ssh.UnknownChannelType, "failed")
			}
		}
	}()

	config := sshTunnelSshConfig{
		fs: testDialSshTunnelFs{
			knownhosts: fmt.Sprintf("%s %s\n", l.Addr().String(), serverHostKeyPub),
		},
		user: "guest",
		idents: []string{
			"/id_ed25519",
		},
		addr:       l.Addr().String(),
		knownHosts: "/known_hosts",
	}
	_ = config
	tun, err := dialSshTunnel(config, ":22")
	if err != nil {
		t.Fatal(err)
	}
	defer tun.Close()

	if _, err := tun.Write([]byte("OK")); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 2, 2)
	if _, err := io.ReadFull(tun, b); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b, []byte("OK")) {
		t.Fatal(b)
	}
}
