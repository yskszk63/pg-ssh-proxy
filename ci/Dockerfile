# syntax=docker/dockerfile:1.3-labs
FROM ubuntu:latest

RUN <<eot
apt update
DEBIAN_FRONTEND=noninteractive apt install openssh-server --no-install-recommends -y
apt clean
rm -rf /var/lib/apt/lists/*
mkdir /run/sshd
mkdir ~/.ssh
chmod 700 ~/.ssh
touch ~/.ssh/known_hosts
touch ~/.ssh/authorized_keys
mkdir /conf
eot

VOLUME /conf
ADD entrypoint.sh /

CMD /bin/bash /entrypoint.sh
