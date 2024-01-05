FROM golang:1.20

ENV TZ="Europe/Berlin"

WORKDIR /usr/src/app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -o /usr/local/bin/energystore -ldflags="-s -w" server.go
RUN go build -o /usr/local/bin/initQoV -ldflags="-s -w" initQoV.go
RUN go build -o /usr/local/bin/ebowctl -ldflags="-s -w" ebowctl.go
RUN go build -o /usr/local/bin/estore -ldflags="-s -w" estore.go


COPY config.yaml /etc/energystore/

RUN rm -r ./*

VOLUME /opt/rawdata

EXPOSE 8080

CMD ["energystore", "-configPath", "/etc/energystore/", "-logtostderr=true", "-stderrthreshold=INFO"]