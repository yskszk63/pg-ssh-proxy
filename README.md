# pg-ssh-proxy

Postgresql SSH tunneling proxy server.

Work In Progress..

## Usage

```
Usage of pg-ssh-proxy:
  -addr string
        listen address. (default "[::1]:5432")
  -config string
        config file. (default "~/.config/pg-ssh-proxy.toml")
```

## Config

`~/.config/pg-ssh-proxy.toml`

```toml
[postgres]
addr = "10.88.0.2:5432"
#dbname = "postgres" # DEFAULT: SAME as entry name.

[postgres.ssh]
addr = "10.88.0.3:22"
#user = "guest" # DEFAULT: uid.
#identity = [ # DEFAULT: ~/.ssh/id_rsa, ~/.ssh/id_ed25519
#    "~/.ssh/id_rsa",
#    "~/.ssh/id_ed25519",
#]
#known_hosts = "~/.ssh/known_hosts" # DEFAULT: ~/.ssh/known_hosts
```

# License

[MIT](LICENSE)
