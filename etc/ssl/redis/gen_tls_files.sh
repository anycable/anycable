#!/bin/bash

# Generate some test certificates which are used by the regression test suite:
#
#   files/ca.{crt,key}          Self signed CA certificate.
#   files/redis.{crt,key}       A certificate with no key usage/policy restrictions.
#   files/client.{crt,key}      A certificate restricted for SSL client usage.
#   files/server.{crt,key}      A certificate restricted for SSL server usage.
#   files/redis.dh              DH Params file.

generate_cert() {
    local name=$1
    local cn="$2"
    local opts="$3"

    local keyfile=files/${name}.key
    local certfile=files/${name}.crt

    [ -f $keyfile ] || openssl genrsa -out $keyfile 2048
    openssl req \
        -new -sha256 \
        -subj "/O=Redis Test/CN=$cn" \
        -key $keyfile | \
        openssl x509 \
            -req -sha256 \
            -CA files/ca.crt \
            -CAkey files/ca.key \
            -CAserial files/ca.txt \
            -CAcreateserial \
            -days 365 \
            $opts \
            -out $certfile
}

mkdir -p files
[ -f files/ca.key ] || openssl genrsa -out files/ca.key 4096
openssl req \
    -x509 -new -nodes -sha256 \
    -key files/ca.key \
    -days 3650 \
    -subj '/O=Redis Test/CN=Certificate Authority' \
    -out files/ca.crt

cat > files/openssl.cnf <<_END_
[ server_cert ]
keyUsage = digitalSignature, keyEncipherment
nsCertType = server

[ client_cert ]
keyUsage = digitalSignature, keyEncipherment
nsCertType = client
_END_

generate_cert server "Server-only" "-extfile files/openssl.cnf -extensions server_cert"
generate_cert client "Client-only" "-extfile files/openssl.cnf -extensions client_cert"
generate_cert redis "Generic-cert"

[ -f files/redis.dh ] || openssl dhparam -out files/redis.dh 2048
