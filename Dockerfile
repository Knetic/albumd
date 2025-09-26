FROM debian:12.9

WORKDIR /var/lib/albumd

VOLUME /usr/share/albumd
EXPOSE 8080

COPY ./.bin/albumd /usr/local/bin/albumd
COPY ./templates /var/lib/albumd/templates

CMD ["/usr/local/bin/albumd", "-path", "/usr/share/albumd", "-thumbs", "/usr/share/albumd/thumbs"]