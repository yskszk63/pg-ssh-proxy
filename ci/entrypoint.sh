#!/bin/bash

set -e

[ -f /conf/known_hosts ] && cat /conf/known_hosts > ~/.ssh/known_hosts
[ -f /conf/authorized_keys ] && cat /conf/authorized_keys > ~/.ssh/authorized_keys
/usr/sbin/sshd -D
