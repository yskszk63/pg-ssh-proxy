package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path"

	"github.com/BurntSushi/toml"
	"github.com/adrg/xdg"
	homedir "github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/sync/errgroup"
)

type SshConnection struct {
	Addr       string   `toml:"addr"`
	User       string   `toml:"user"`
	Identity   []string `toml:"identity"`
	KnownHosts string   `toml:"known_hosts"`
}

type Connection struct {
	Addr   string        `toml:"addr"`
	Dbname string        `toml:"dbname"`
	Ssh    SshConnection `toml:"ssh"`
}

type Config struct {
	Connections map[string]*Connection
}

func parseConfig(path string) (*Config, error) {
	r := Config{
		Connections: map[string]*Connection{},
	}

	fp, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	dec := toml.NewDecoder(fp)
	if _, err := dec.Decode(&r.Connections); err != nil {
		return nil, err
	}

	for name, conf := range r.Connections {
		if conf.Dbname == "" {
			conf.Dbname = name
		}
		if conf.Ssh.User == "" {
			if u, _ := user.Current(); u != nil {
				conf.Ssh.User = u.Username
			}
		}
		if conf.Ssh.Identity == nil {
			conf.Ssh.Identity = []string{
				"~/.ssh/id_rsa",
				"~/.ssh/id_ed25519",
			}
		}
		if conf.Ssh.KnownHosts == "" {
			conf.Ssh.KnownHosts = "~/.ssh/known_hosts"
		}
	}

	return &r, nil
}

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

	pkt := make([]byte, size-4)
	if n, err := io.ReadFull(r, pkt); err != nil {
		return nil, err
	} else if n != int(size-4) {
		return nil, fmt.Errorf("%d != 4", size-4)
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
	if err := write32(w, uint32(len(pkt)+4)); err != nil {
		return err
	}

	if n, err := w.Write(pkt); err != nil {
		return err
	} else if n != len(pkt) {
		return fmt.Errorf("%d != n", len(pkt))
	}

	return nil
}

func serve(cx context.Context, conn net.Conn, config *Config) error {
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
	var addrFlag = flag.String("addr", "[::1]:5432", "listen address.")
	var configFlag = flag.String("config", path.Join(xdg.ConfigHome, "pg-ssh-proxy.toml"), "config file.")
	flag.Parse()

	config, err := parseConfig(*configFlag)
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
