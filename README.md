# Default Deployment and Setup Guide

## 1. Create Couchbase User

Create a user named `EMX` in **Couchbase Server** with the role:

- `ro_admin`

---

## 2. Generate Certificates

### 2.a. Client Certificates
- Generate a private key (`key.pem`) for the client.
- Request a certificate (`cert.pem`) from the **Couchbase Server CA** for the user `EMX`.

### 2.b. Server Certificates
- Generate a private key and certificate for **EMX server TLS connections**.

---

## 3. Set Environment Variables

```bash
export CB_PROTOCOL=<https/http>
export CB_HOST=<Couchbase server hostname>
export CB_PORT=<Couchbase port number>
export EMX_PORT=<EMX server port number>
export EMX_TLS_KEY=<path to EMX server TLS key>
export EMX_TLS_CERT=<path to EMX server TLS cert>
```

**Defaults:**

- `CB_PROTOCOL`: `https`
- `CB_HOST`: `localhost`
- `CB_PORT`: `18091`
- `EMX_PORT`: `9876`
- `EMX_TLS_KEY`: `""`
- `EMX_TLS_CERT`: `""`

---

## 4. Verify Certificate and Key Files

Ensure all required certificate and key files exist in the correct paths in your local working directory.

---

## 5. Launch the Service

TLS is enabled by default. Use the `--disableTLS` flag to run in HTTP mode.
Certificate and Key file paths can be set via flags instead of ENV variables.

### 5.a. Native Execution

```bash
go run exporter/main.go \
  [--clientCert cert.pem] \
  [--clientKey key.pem] \
  [--tlsCert server.crt] \
  [--tlsKey server.key] \
  [--disableTLS]
```

### 5.b. Docker Execution

```bash
docker run --name couchbase_emx -p $EMX_PORT:$EMX_PORT \
  -e CB_HOST=$CB_HOST \
  -e CB_PROTOCOL=$CB_PROTOCOL \
  -e CB_PORT=$CB_PORT \
  -e EMX_PORT=$EMX_PORT \
  -e EMX_TLS_KEY=$EMX_TLS_KEY \
  -e EMX_TLS_CERT=$EMX_TLS_CERT \
  couchbase-emx \
  [--clientCert cert.pem] \
  [--clientKey key.pem] \
  [--tlsCert server.crt] \
  [--tlsKey server.key] \
  [--disableTLS]
```

---

## 6. Configure Prometheus

Add the following job to your Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'couchbase_emx'
    static_configs:
      - targets: ['<emx machine hostname>:9876']
        labels:
          cluster_name: '<Cluster name>'
```

Replace:

- `<emx machine hostname>` with the hostname of the machine running the EMX exporter
- `<Cluster name>` with the name of your Couchbase cluster

---
