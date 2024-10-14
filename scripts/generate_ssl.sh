#!/bin/bash
set -e

# Define variables
CERT_DIR=/etc/postgresql/ssl
SERVER_KEY=$CERT_DIR/server.key
SERVER_CSR=$CERT_DIR/server.csr
SERVER_CRT=$CERT_DIR/server.crt
ROOT_CA_KEY=$CERT_DIR/rootCA.key
ROOT_CA_CRT=$CERT_DIR/rootCA.crt

# Create certificate directory
mkdir -p $CERT_DIR

# Generate server key
openssl genrsa -out $SERVER_KEY 2048

# Generate server certificate signing request (CSR)
openssl req -new -key $SERVER_KEY -out $SERVER_CSR -subj "/C=US/ST=State/L=City/O=Organization/OU=OrgUnit/CN=example.com"

# Generate self-signed certificate
openssl x509 -req -days 365 -in $SERVER_CSR -signkey $SERVER_KEY -out $SERVER_CRT

# Generate root CA key
openssl genrsa -out $ROOT_CA_KEY 2048

# Generate root CA certificate
openssl req -x509 -new -nodes -key $ROOT_CA_KEY -sha256 -days 1024 -out $ROOT_CA_CRT -subj "/C=US/ST=State/L=City/O=Organization/OU=OrgUnit/CN=root"

# Fix permissions
chmod 600 $SERVER_KEY
