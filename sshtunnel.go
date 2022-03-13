package main

import (
	"io"
	"io/fs"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type sshTunnelSshConfig struct {
	fs         fs.FS
	user       string
	addr       string
	idents     []string
	knownHosts string
}

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

func dialSshTunnel(config sshTunnelSshConfig, addr string) (*sshTunnel, error) {
	signers := make([]ssh.Signer, 0, len(config.idents))
	for _, ident := range config.idents {
		pem, err := fs.ReadFile(config.fs, ident)
		if err != nil {
			return nil, err
		}
		signer, err := ssh.ParsePrivateKey(pem)
		if err != nil {
			return nil, err
		}
		signers = append(signers, signer)
	}

	kh, err := func() (ssh.HostKeyCallback, error) {
		// knownhosts.New -> no fs.FS input.
		fp, err := os.CreateTemp("", "known_hosts")
		if err != nil {
			return nil, err
		}
		defer os.Remove(fp.Name())

		src, err := config.fs.Open(config.knownHosts)
		if err != nil {
			return nil, err
		}
		defer src.Close()

		if _, err := io.Copy(fp, src); err != nil {
			return nil, err
		}

		return knownhosts.New(fp.Name())
	}()
	if err != nil {
		return nil, err
	}

	sshconf := ssh.ClientConfig{
		User: config.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signers...),
		},
		HostKeyCallback: kh,
	}
	client, err := ssh.Dial("tcp", config.addr, &sshconf)
	if err != nil {
		return nil, err
	}

	conn, err := client.Dial("tcp", addr)
	if err != nil {
		client.Close()
		return nil, err
	}

	return &sshTunnel{
		client: client,
		conn:   conn,
	}, nil
}
