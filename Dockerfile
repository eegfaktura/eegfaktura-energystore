FROM golang:1.20

WORKDIR /usr/src/app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -o /usr/local/bin/energystore -ldflags="-s -w"

COPY config.yaml /etc/energystore/

RUN rm -r ./*

VOLUME /opt/rawdata

EXPOSE 8080

CMD ["energystore", "-configPath", "/etc/energystore/", "-v=3", "-logtostderr=true", "-stderrthreshold=INFO"]