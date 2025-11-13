#!/bin/sh

inotifyd /usr/bin/nginx-reload.sh \
  /etc/nginx/authorized-tenants/authorized_tenants.map:c \
  /etc/nginx/secrets/.htpasswd:c &
