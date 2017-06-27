## Instance Identity

**Note** This feature is experimental

Diego's instance identity system provides each app container with a
certificate and private key.  You can find the file path
for each by inspecting the environment variables `CF_INSTANCE_CERT` and `CF_INSTANCE_KEY`.
The files are PEM encoded.

### About the Certificate
- The certificate's `Common Name` property is set to the instance id.
- The SAN is set to the container IP address that is running the given app instance.
- On Cloud Foundry, the certificate's `Organizational Unit` property is set to the string `app:app-guid`
  where `app-guid` is the Application GUID assigned by Cloud Controller.

The certificate expires 24 hours after the container is created.

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
