# https://blog.codeship.com/building-minimal-docker-containers-for-go-applications
FROM scratch
ADD .docker/ca-certificates.crt /etc/ssl/certs/

ADD .docker/anycable-go /usr/local/bin/

ENV ADDR "0.0.0.0:8080"

EXPOSE 8080

CMD ["anycable-go"]
