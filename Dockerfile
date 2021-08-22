FROM golang:latest as builder

RUN mkdir gool

COPY .git gool/.git
COPY go.* gool/
COPY Makefile gool/
COPY build gool/build
COPY src gool/src

RUN cd gool && make

FROM golang:latest

RUN groupadd -r gool \
     && useradd --no-log-init -r -g gool gool \
     && mkdir -p /opt/gool \
     && chown gool -R /opt/gool \
     && chgrp gool -R /opt/gool

WORKDIR /opt/gool

USER gool:gool

COPY LICENSE ./
COPY --from=builder --chown=gool:gool /go/gool/build/bin bin

CMD /opt/gool/bin/node
