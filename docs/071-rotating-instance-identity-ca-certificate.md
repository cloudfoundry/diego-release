---
title: Rotating Instance Identity CA Certificates
expires_at : never
tags: [diego-release]
---

# Rotating Instance Identity CA Certificates

Instance identity certificate provides each application with PEM encoded
certificate and key that uniquely encodes its identity in CF deployment. See
more about it
[here](https://docs.cloudfoundry.org/adminguide/instance-identity.html).

To update the CA that signs these certificates the recommended way is through
2-step deployment. In first deployment introduce new instance identity CA
certificate and add it to the list of trusted certs:

* ssh_proxy - `backends.tls.ca_certificates`
* gorouter - `router.ca_certs`
* cflinuxfs3-rootfs-setup - `cflinuxfs3-rootfs.trusted_certs`
* rep - `containers.trusted_ca_certificates`
* credhub - `credhub.authentication.mutual_tls.trusted_cas`

Replace rep `diego.executor.instance_identity_ca_cert` and
`diego.executor.instance_identity_key` with new diego certificate and key.

In subsequent deploy remove old diego instance identity certificate and its ca.
