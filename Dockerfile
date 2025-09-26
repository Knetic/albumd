FROM debian:12.9

VOLUME /usr/share/albumd
EXPOSE 8080

WORKDIR /usr/share/albumd

COPY ./.bin/albumd /usr/local/bin/albumd
COPY ./templates /usr/share/albumd/templates

CMD ["/usr/local/bin/albumd"]