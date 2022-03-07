package main

import (
	"fmt"
	"io/fs"
	"os/user"
	"regexp"

	"github.com/BurntSushi/toml"
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
	fs          fs.FS
	Connections map[string]*Connection
}

var hasPort = regexp.MustCompile(`:\d+$`)

func clarifyKnownPort(addr string, kp int16) string {
	if hasPort.MatchString(addr) {
		return addr
	}
	return fmt.Sprintf("%s:%d", addr, kp)
}

func parseConfig(fs fs.FS, path string) (*Config, error) {
	r := Config{
		fs:          fs,
		Connections: map[string]*Connection{},
	}

	if _, err := toml.DecodeFS(fs, path, &r.Connections); err != nil {
		return nil, err
	}

	for name, conf := range r.Connections {
		if conf.Addr == "" {
			return nil, fmt.Errorf("requires: `addr`")
		}
		conf.Addr = clarifyKnownPort(conf.Addr, 5432)
		if conf.Dbname == "" {
			conf.Dbname = name
		}

		if conf.Ssh.Addr == "" {
			return nil, fmt.Errorf("requires: `ssh.addr`")
		}
		conf.Ssh.Addr = clarifyKnownPort(conf.Ssh.Addr, 22)
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
