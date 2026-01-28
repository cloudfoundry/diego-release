---
title: Usage
expires_at : never
tags: [diego-release, localdriver]
---

# Usage of localdriver:
----
```
  -caFile string
        the certificate authority public key file to use with ssl authentication
  -certFile string
        the public key file to use with ssl authentication
  -clientCertFile string
        the public key file to use with client ssl authentication
  -clientKeyFile string
        the private key file to use with client ssl authentication
  -debugAddr string
        host:port for serving pprof debugging info
  -driversPath string
        Path to directory where drivers are installed
  -insecureSkipVerify
        whether SSL communication should skip verification of server IP addresses in the certificate
  -keyFile string
        the private key file to use with ssl authentication
  -listenAddr string
        host:port to serve volume management functions (default "0.0.0.0:9750")
  -logLevel string
        log level: debug, info, error or fatal (default "info")
  -mountDir string
        Path to directory where fake volumes are created (default "/tmp/volumes")
  -requireSSL
        whether the fake driver should require ssl-secured communication
  -transport string
        Transport protocol to transmit HTTP over (default "tcp")
```
