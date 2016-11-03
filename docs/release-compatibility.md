## Release Compatibility

Diego releases are tested against Cloud Foundry, Garden. Compatible versions of
Garden are listed with Diego on the
[Github releases page](https://github.com/cloudfoundry/diego-release/releases).

### Checking out a release of Diego

The Diego git repository is tagged with every release. To move the git repository
to match a release, do the following:

```bash
cd diego-release/
# checking out release v0.1437.0
git checkout v0.1437.0
./scripts/update
git clean -ffd
```

### From a final release of CF

On the CF Release
[GitHub Releases](https://github.com/cloudfoundry/cf-release/releases) page,
recommended versions of Diego, and Garden are listed with each CF Release.
This is the easiest way to correlate releases.

Alternatively, you can use records of CF and Diego compatibility captured from
automated testing. First look up the release candidate SHA for your CF release.
This is listed as the `commit_hash` in the release yaml file. Find the SHA in
[diego-cf-compatibility/compatibility-v2.csv](https://github.com/cloudfoundry/diego-cf-compatibility/blob/master/compatibility-v2.csv)
to look up tested versions of Diego Release, Garden. For old versions of
diego-release, you have to make sure you are using a compatible version of ETCD
as well.

Example: Let's say you want to deploy Diego alongside CF final release `222`. The release file
[`releases/cf-222.yml`](https://github.com/cloudfoundry/cf-release/blob/master/releases/cf-222.yml)
in the cf-release repository contains the line `commit_hash: 53014242`.
Finding `53014242` in `diego-cf-compatibility/compatibility-v2.csv` reveals Diego
0.1437.0, Garden 0.308.0, and ETCD 16 have been verified to be compatible.


### From a specific CF Release commit SHA

Not every cf-release commit will appear in the diego-cf compatibility table,
but many will work with some version of Diego.

If you can't find a specific cf-release SHA in the table, deploy the diego-release
that matches the most recent cf-release relative to that commit. To do this, go back
through cf-release's git log from your commit until you find a Final Release commit
and then look up that commit's SHA in the diego-cf compatibility table.

