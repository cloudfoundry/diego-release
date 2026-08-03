---
title: Create and Mount Options
expires_at : never
tags: [diego-release, localdriver]
---

## Create
```
dockerdriver.CreateRequest{
    Name: "Volume",
    Opts: map[string]interface{}{
        "volume_id": "something_different_than_test",
        "passcode" : "someStringPasscode",              <- OPTIONAL
    },
})
```

## Mount
```
localDriver.Mount(logger, dockerdriver.MountRequest{
    Name: "Volume",
    Opts: map[string]interface{}{
        "passcode":"someStringPasscode"                 <- REQUIRED if used in Create
    },
})
```
