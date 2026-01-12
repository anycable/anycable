docker run --rm \
  -v ./files:/tls:ro \
  -p 6383:6383 \
  redis:7-alpine \
  redis-server \
    --tls-port 6383 \
    --port 0 \
    --tls-cert-file /tls/server.crt \
    --tls-key-file /tls/server.key \
    --tls-ca-cert-file /tls/ca.crt
