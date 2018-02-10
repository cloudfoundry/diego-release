## Release Compatibility

Diego releases are tested against Cloud Foundry, Garden. Compatible versions of
Garden are listed with Diego on the
[Github releases page](https://github.com/cloudfoundry/diego-release/releases).

### Checking out a release of Diego

The Diego git repository is tagged with every release. To move the git repository
to match a release, do the following:

```bash
cd diego-release/
# checking out release v1.0.0
git checkout v1.0.0
./scripts/update
git clean -ffd
```
