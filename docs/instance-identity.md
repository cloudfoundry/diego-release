## Instance Identity

The instance identity system in Diego provides each application instance with a PEM-encoded
[X.509](https://tools.ietf.org/html/rfc5280) certificate and [PKCS#1](https://tools.ietf.org/html/rfc3447) RSA private key.  The values of the environment variables `CF_INSTANCE_CERT` and `CF_INSTANCE_KEY` contain the absolute paths to the certificate and private key files, respectively.


### About the Certificate

- The certificate's `Common Name` property is set to the instance guid.
- The certificate contains an IP SAN set to the container IP address for the given app instance.
- For Cloud Foundry apps, the certificate's `Organizational Unit` property is set to the string `app:app-guid`, where `app-guid` is the application guid assigned by Cloud Controller.

By default, the certificate is valid for the 24 hours after the container is created, but the Diego operator may control this validity period with the `diego.executor.instance_identity_validity_period_in_hours` BOSH property. The smallest allowed validity duration is 1 hour.

The Diego cell rep will supply a new certificate-key pair to the container before the end of the validity period. The new pair of files will replace the existing pair at the same path location, with each file being replaced atomically. If the validity period is greater than 4 hours, the pair will be regenerated between 1 hour and 20 minutes before the end of the validity period. If the validity period is less than or equal to 4 hours, the pair will be regenerated between 1/4 and 1/12 of the way to the end of the period.


### Enabling Instance Identity

Instance Identity can be enabled by setting the following properties in the
deployment manifest:

- `diego.executor.instance_identity_ca_cert`: The CA certificate used to sign the app container's certificate.
- `diego.executor.instance_identity_key`: The private key of the given CA certificate.


### Requirements

The CA certificate must have all the properties required to correctly sign other certificates:

1. `Subject Key Identifier` must be set.
2. `KeyUsage` must include `KeyCertSign`.
3. Intermediate CA certificates should either leave `ExtendedKeyUsage` empty, or assign it the `any` property.
