FROM scratch
ADD .docker/ca-certificates.crt /etc/ssl/certs/
ADD .docker/main /

EXPOSE 8080

CMD ["/main"]
