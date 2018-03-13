## Release Compatibility

Diego releases are tested against Garden-Runc, Garden-Windows, and the other
releases that [cf-deployment](https://github.com/cloudfoundry/cf-deployment)
provides.
Each Diego [GitHub release](https://github.com/cloudfoundry/diego-release/releases)
lists the versions of these resources against which the Diego team
CI pipeline verified that Diego release.


### Checking out a release of Diego

The Diego Git repository is tagged with every release. To update the Git repository
to match a particular release, do the following:

```bash
cd diego-release/
# checking out release v1.0.0
git checkout v1.0.0
./scripts/update
git clean -ffd
```
