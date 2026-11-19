# Service-account instance identity (experimental)

Compatible Cloud Controller versions supply one optional platform-owned
`certificate_properties.service_account` with both `name` and organization `prefix`
for authorized app launches and runtime tasks. Both fields are canonical lowercase
RFC 1123 labels of 3–63 characters; accounts without a prefix are invalid.
BBS validates the identity, retains it in stored LRP run-info assembly, and rep
copies it into executor credential input. Empty certificate properties retain
the existing unbound/staging behavior.

When enabled, executor adds exactly one `<name>.<prefix>.svc.identity` DNS SAN to both
instance-identity and C2C certificates. Instance CN/GUID SAN, IP SAN, caller
app/space/organization OUs, C2C internal routes, usages and validity remain intact.
Each credential and each instance uses independent keys; rotation generates new
keys. Instance GUIDs and internal-route names cannot use the reserved
`svc.identity` namespace, including case-folded and trailing-dot variants.

## Rollout and configuration

Both `rep` and `rep_windows` declare:

```yaml
diego:
  executor:
    service_account_identity_enabled: false
```

The default-off gate is independent of the CAPI provisioning/runtime gates.
Enabling it requires the existing instance-identity CA/key, credential directory
and positive validity period. Cells with the gate disabled reject assigned
account credentials instead of silently omitting the account SAN. Credential
managers without instance-identity support reject assigned accounts as well.

Keep the CAPI runtime gate disabled until every participating BBS/rep/cell runs
the compatible contract and cell configuration. Older software can discard unknown
protobuf fields; this POC has no automatic foundation-wide capability negotiation
or auction placement capability advertisement. The operator rollout gate must
therefore cover all participating cells, including isolation segments and Windows.

## Launch and lifecycle semantics

The credential runner snapshots account identity at launch. Later assignment
changes require a new launch/restart. Timed renewal and route-triggered C2C
regeneration retain that snapshot, while route/IP updates remain live. An initially
unbound launch cannot acquire account identity by changing container metadata.
Disabling/unbinding does not revoke certificates already issued to a workload.

CAPI excludes account identity from staging and captures runtime task/process
assignments. Diego consumes the trusted typed contract rather than environment
variables or user-supplied SANs. Org-owned account authorization, same-org space entitlements and
UAA client reconciliation remain CAPI responsibilities.

## Local contract provenance

The POC pins `code.cloudfoundry.org/bbs/models` to
`v1.15.1-0.20261119082436-7fc64d1d37a1`, from the separately versioned BBS
source SHA `7fc64d1d37a1bb2f5bcb6619c87ac27f859ea62e`. Go bindings were generated
from the authoritative protobuf and vendor was populated through `go mod vendor`.
The revision is published on `github.com/rkoster/bbs/models` and selected through
an explicit versioned `replace` with committed checksums. Vendored and module-mode
builds use that same source without a local proxy. Remove the fork replacement
and select the upstream module version once BBS PR #168 is merged and published.

Local verification covers protobuf-to-rep-to-real-certificate flow for two apps
and a runtime task, independent keys, renewal, unbind invariance, malformed and
injected identity denial, cell gates, and both job templates. Live scheduling,
staging, UAA token acquisition and Windows execution still require lab evidence.
