FROM debian:12.9

VOLUME /usr/share/albumd
EXPOSE 8080

WORKDIR /usr/share/albumd

COPY ./.bin/albumd /usr/local/bin/albumd

CMD ["/usr/local/bin/albumd"]