## Data Store Encryption

Diego Release must be configured with a set of encryption keys to be used when
encrypting sensitive data at rest in the BBS data store. To configure encryption the
`diego.bbs.encryption_keys` and `diego.bbs.active_key_label` properties should
be set.

Diego will automatically (re-)encrypt all of the stored data using the
active key upon boot. This ensures an operator can rotate a key out without
having to manually rewrite all of the records.

### Configuring Encryption Keys

Diego uses multiple keys for decryption while allowing only one for encryption.
This allows an operator to rotate encryption keys without downtime.

For example:

```yaml
properties:
  diego:
    bbs:
      active_key_label: key-2015-09
      encryption_keys:
      - label: 'key-2015-09'
        passphrase: 'my september passphrase'
      - label: 'key-2015-08'
        passphrase: 'my august passphrase'
```

In the above example, the operator has configured two encryption keys and selected one of them to be the active key. The active key is the one used for encryption, while all the
others can be used for decryption.

The key labels must be no longer than 127 characters, while the passphrases
have no enforced limit. Additionally, the key label must not contain a
`:` (colon) character, as the command-line flags to the BBS process use `:`
to separate the key label from the passphrase.
