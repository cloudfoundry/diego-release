---
title: Building from Source
expires_at : never
tags: [diego-release, cfdot]
---

## Building from Source

`cfdot` requires the [Diego BBS client library](https://github.com/cloudfoundry/bbs).
If you have already cloned [diego-release](https://github.com/cloudfoundry/diego-release),
you can run the following commands using that diego-release directory as your
GOPATH.  Alternatively, run these commands with any other GOPATH and `go get`
will automatically fetch the latest BBS code from diego-release.

```bash
# Get cfdot and required dependencies
go get code.cloudfoundry.org/cfdot
cd src/code.cloudfoundry.org/cfdot

# Build for Linux
GOOS=linux go build .

# Build for Mac
GOOS=darwin go build .

# Build for Windows
GOOS=windows go build .
```
