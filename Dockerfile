FROM golang:latest as builder

RUN mkdir -p /go/gool

COPY ./go.* /go/gool/
COPY ./Makefile /go/gool/
COPY ./build /go/gool/build
COPY ./src /go/gool/src

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
