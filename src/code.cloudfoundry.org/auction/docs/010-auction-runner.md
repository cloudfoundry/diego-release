---
title: The Auction Runner
expires_at : never
tags: [diego-release, auction]
---

# The Auction Runner

The `auctionrunner` package provides an [ifrit process
runner](https://github.com/tedsuo/ifrit/blob/master/runner.go) which consumes
an incoming stream of requested auction work, batches it up, communicates with
the Cell reps, picks winners, and then instructs the Cells to perform the work.
