FROM golang:1.27

ENV TZ="Europe/Berlin"

WORKDIR /usr/src/app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -o /usr/local/bin/energystore -ldflags="-s -w" server.go
# RUN go build -o /usr/local/bin/initQoV -ldflags="-s -w" initQoV.go
# RUN go build -o /usr/local/bin/ebowctl -ldflags="-s -w" ebowctl.go
RUN go build -o /usr/local/bin/estore -ldflags="-s -w" estore.go

COPY config.yaml /etc/energystore/

RUN rm -r ./*

# Nicht als root laufen (Befund A06 der Altsystem-Analyse).
# Der Dienst lauscht auf 8080, also oberhalb von 1024, und braucht keine
# Privilegien. Die Badger-Daten liegen auf einem PVC - dort regelt fsGroup im
# Deployment die Gruppenzugehoerigkeit; /opt/rawdata ist ein anonymes Volume und
# muss hier gehoeren, BEVOR die VOLUME-Anweisung kommt (spaetere Aenderungen
# landen nicht mehr im Image).
RUN groupadd -g 1000 app \
 && useradd -u 1000 -g app -M -s /usr/sbin/nologin app \
 && mkdir -p /opt/rawdata /opt/energy \
 && chown -R app:app /opt/rawdata /opt/energy

VOLUME /opt/rawdata

USER app

EXPOSE 8080

CMD ["energystore", "-configPath", "/etc/energystore/", "-logtostderr=true", "-stderrthreshold=INFO"]
