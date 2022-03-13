package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path"

	"github.com/adrg/xdg"
	homedir "github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/sync/errgroup"
)

type sshTunnel struct {
	client *ssh.Client
	conn   net.Conn
}

func (s *sshTunnel) Close() error {
	if err := s.conn.Close(); err != nil {
		return err
	}
	if err := s.client.Close(); err != nil {
		return err
	}
	return nil
}

func (s *sshTunnel) Read(b []byte) (int, error) {
	return s.conn.Read(b)
}

func (s *sshTunnel) Write(b []byte) (int, error) {
	return s.conn.Write(b)
}

func connect(config *Connection) (*sshTunnel, error) {
	signers := make([]ssh.Signer, 0)
	for _, fname := range config.Ssh.Identity {
		ident, err := homedir.Expand(fname)
		if err != nil {
			return nil, err // FIXME ignore?
		}
		sshkey, err := os.ReadFile(ident)
		if err != nil {
			return nil, err // FIXME ignore?
		}
		signer, err := ssh.ParsePrivateKey(sshkey)
		if err != nil {
			return nil, err // FIXME ignore?
		}
		signers = append(signers, signer)
	}

	hk, err := homedir.Expand(config.Ssh.KnownHosts)
	if err != nil {
		return nil, err
	}
	hkcb, err := knownhosts.New(hk)
	if err != nil {
		return nil, err
	}

	sshconf := ssh.ClientConfig{
		User: config.Ssh.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signers...),
		},
		HostKeyCallback: hkcb,
	}
	client, err := ssh.Dial("tcp", config.Ssh.Addr, &sshconf)
	if err != nil {
		return nil, err
	}

	conn, err := client.Dial("tcp", config.Addr)
	if err != nil {
		client.Close()
		return nil, err
	}

	return &sshTunnel{
		client: client,
		conn:   conn,
	}, nil
}

func serve(cx context.Context, conn net.Conn, config *Config) error {
	var up *sshTunnel
	for up == nil {
		var pkt rawInitialPacket
		if err := pkt.read(conn); err != nil {
			return err
		}

		p2, err := pkt.toConcrete()
		if err != nil {
			return err
		}

		switch p := p2.(type) {
		case *startupMessage:
			var entry *Connection

			if db := p.database(); db != nil {
				entry = config.Connections[*db]
			}

			if entry == nil {
				return fmt.Errorf("No such connection.")
			}
			up, err = connect(entry)
			if err != nil {
				return err
			}

			raw := p.toRaw()
			if err := raw.write(up); err != nil {
				up.Close()
				return err
			}

		case *sslRequest:
			if _, err := conn.Write([]byte("N")); err != nil {
				return err
			}
		}
	}
	defer up.Close()

	eg, _ := errgroup.WithContext(cx)
	eg.Go(func() error {
		_, err := io.Copy(up, conn)
		return err
	})
	eg.Go(func() error {
		_, err := io.Copy(conn, up)
		return err
	})
	return eg.Wait()
}

type osfs struct{}

func (osfs) Open(name string) (fs.File, error) {
	return os.Open(name)
}

func main() {
	var addrFlag = flag.String("addr", "[::1]:5432", "listen address.")
	var configFlag = flag.String("config", path.Join(xdg.ConfigHome, "pg-ssh-proxy.toml"), "config file.")
	flag.Parse()

	config, err := parseConfig(osfs{}, *configFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(-1)
	}

	l, err := net.Listen("tcp", *addrFlag)
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
			if err := serve(context.TODO(), conn, config); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}()
	}
}
