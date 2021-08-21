FROM golang:latest

RUN groupadd -r gool \
     && useradd --no-log-init -r -g gool gool \
     && mkdir -p /opt/gool \
     && chown gool -R /opt/gool \
     && chgrp gool -R /opt/gool

WORKDIR /opt/gool

USER gool:gool


