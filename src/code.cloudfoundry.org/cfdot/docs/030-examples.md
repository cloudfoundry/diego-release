---
title: Examples
expires_at : never
tags: [diego-release, cfdot]
---

## Examples

```bash
# count the total number of desired instances
$ cfdot desired-lrp-scheduling-infos | jq '.instances' | jq -s 'add'
568

# show instance counts by state
$ cfdot actual-lrps | jq -s -r 'group_by(.state)[] | .[0].state + ": " + (length | tostring)'
CRASHED: 36
RUNNING: 531
UNCLAIMED: 1
```
