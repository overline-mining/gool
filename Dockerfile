FROM golang:latest as builder

RUN mkdir -p /go/gool /go/gool/build /go/gool/src

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

RUN mkdir -p /opt/gool/bin
COPY LICENSE ./
COPY --from=builder --chown=gool:gool /go/gool/build/bin /opt/gool/bin

CMD /opt/gool/bin/node
