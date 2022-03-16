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
	"golang.org/x/sync/errgroup"
)

func proxy(cx context.Context, c1, c2 io.ReadWriter) error {
	eg, _ := errgroup.WithContext(cx)
	eg.Go(func() error {
		_, err := io.Copy(c1, c2)
		return err
	})
	eg.Go(func() error {
		_, err := io.Copy(c2, c1)
		return err
	})
	return eg.Wait()
}

func serve(cx context.Context, conn net.Conn, config *config) error {
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
				c, exists := config.Connections[*db]
				if exists {
					entry = c
				}
			}

			if entry == nil {
				return fmt.Errorf("No such connection.")
			}
			up, err = dialSshTunnel(sshTunnelSshConfig{
				fs:         osfs{},
				user:       entry.Ssh.User,
				addr:       entry.Ssh.Addr,
				idents:     entry.Ssh.Identity,
				knownHosts: entry.Ssh.KnownHosts,
			}, entry.Addr)
			if err != nil {
				return err
			}

			p.setDataabse(entry.Dbname)
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

	return proxy(cx, conn, up)
}

type osfs struct{}

func (osfs) Open(name string) (fs.File, error) {
	if fname, err := homedir.Expand(name); err == nil {
		return os.Open(fname)
	}
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
				pkt := &errorResponse{
					fields: []errorResponseField{
						{
							code:  'S',
							value: "ERROR",
						},
						{
							code:  'C',
							value: "XX000",
						},
						{
							code:  'M',
							value: err.Error(),
						},
						{
							code:  'R',
							value: "pg-ssh-proxy",
						},
					},
				}
				raw := pkt.toRaw()
				if err := raw.write(conn); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
				fmt.Fprintln(os.Stderr, err)
			}
		}()
	}
}
