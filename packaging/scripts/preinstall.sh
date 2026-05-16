#!/bin/sh
set -eu

if ! getent group orivis >/dev/null 2>&1; then
    groupadd --system orivis
fi

if ! getent passwd orivis >/dev/null 2>&1; then
    useradd --system \
        --gid orivis \
        --home-dir /var/lib/orivis \
        --shell /usr/sbin/nologin \
        --no-create-home \
        orivis
fi

install -d -o orivis -g orivis -m 0750 /var/lib/orivis
