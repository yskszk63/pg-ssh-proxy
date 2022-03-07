package main

import (
	"bytes"
	"context"
	"encoding/binary"
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

func serve(cx context.Context, conn net.Conn, config *Config) error {
	for {
		var pkt rawPacket
		if err := pkt.read(conn); err != nil {
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

		kv := bytes.Split(pkt[4:], []byte{0})
		if len(kv)%2 != 0 {
			return fmt.Errorf("Unexpected format.")
		}
		var entry *Connection
		for i := 0; i < len(kv)/2; i++ {
			// decide upstream
			k := kv[(i*2)+0]
			v := kv[(i*2)+1]

			if string(k) == "database" {
				entry = config.Connections[string(v)]
				break
			}
		}
		if entry == nil {
			return fmt.Errorf("No such connection.")
		}

		signers := make([]ssh.Signer, 0)
		for _, fp := range entry.Ssh.Identity {
			ident, err := homedir.Expand(fp)
			if err != nil {
				return err
			}
			sshkey, err := os.ReadFile(ident)
			if err != nil {
				return err
			}
			signer, err := ssh.ParsePrivateKey(sshkey)
			if err != nil {
				return err
			}
			signers = append(signers, signer)
		}

		kh, err := homedir.Expand(entry.Ssh.KnownHosts)
		if err != nil {
			return err
		}
		hkcb, err := knownhosts.New(kh)
		if err != nil {
			return err
		}

		sshconf := ssh.ClientConfig{
			User: entry.Ssh.User,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signers...),
			},
			HostKeyCallback: hkcb,
		}
		client, err := ssh.Dial("tcp", entry.Ssh.Addr, &sshconf)
		if err != nil {
			return err
		}
		defer client.Close()

		up, err := client.Dial("tcp", entry.Addr)
		if err != nil {
			return err
		}
		defer up.Close()

		if err := pkt.write(up); err != nil {
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
