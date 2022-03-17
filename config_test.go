package main

import (
	"bytes"
	"embed"
	"os/user"
	"reflect"
	"testing"

	"github.com/BurntSushi/toml"
)

//go:embed config_test/*
var dummy embed.FS

func TestParseConfig(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		path  string
		wants map[string]*Connection
		err   string
	}{
		{
			name:  "empty",
			path:  "config_test/empty.toml",
			wants: map[string]*Connection{},
		},
		{
			name: "simple",
			path: "config_test/simple.toml",
			wants: map[string]*Connection{
				"simple": {
					Addr:   "10.20.30.40:5432",
					Dbname: "simple",
					Ssh: sshConnection{
						Addr: "10.20.30.40:22",
						User: u.Username,
						Identity: []string{
							"~/.ssh/id_rsa",
							"~/.ssh/id_ed25519",
						},
						KnownHosts: "~/.ssh/known_hosts",
					},
				},
			},
		},
		{
			name: "not_found",
			path: "config_test/not_exists.toml",
			err:  `open config_test/not_exists.toml: file does not exist`,
		},
		{
			name: "invalid",
			path: "config_test/invalid.toml",
			err:  `toml: line 1: expected '.' or ']' to end table name, but got '\\' instead`,
		},
		{
			name: "no_addr",
			path: "config_test/no_addr.toml",
			err:  "requires: `addr`",
		},
		{
			name: "no_ssh_addr",
			path: "config_test/no_ssh_addr.toml",
			err:  "requires: `ssh.addr`",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := parseConfig(dummy, test.path)
			if test.err != "" && test.err != err.Error() {
				t.Fatal(err)
			}
			if err != nil && test.err == "" {
				t.Fatal(err)
			}

			if test.wants != nil && !reflect.DeepEqual(test.wants, actual.Connections) {
				w := bytes.Buffer{}
				a := bytes.Buffer{}
				if err := toml.NewEncoder(&w).Encode(test.wants); err != nil {
					t.Fatal(err)
				}
				if err := toml.NewEncoder(&a).Encode(actual.Connections); err != nil {
					t.Fatal(err)
				}
				t.Fatalf("%s != %s", w.Bytes(), a.Bytes())
			}
		})
	}
}
