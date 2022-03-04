# pg-ssh-proxy

Work In Progress..

## Config

`~/.config/pg-ssh-proxy.toml`

```toml
[[connections]]
sshaddr = "[::1]:22"
name = "postgres"
username = "user"
identity = "~/.ssh/id_ed25519"
pgaddr = "[::1]:25432"
```

# License

[MIT](LICENSE)
