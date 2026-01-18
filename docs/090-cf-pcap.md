# cf-pcap

## Enable the feature platform wide

Set the rep property `diego.rep.enable_cf_pcap` to true (defaults to false) and
re-deploy diego.

This will set a file capability on the cf-pcap binary which is always inside the
app container to allow the binary to perform packet captures.

## Usage

The binary is placed at `/tmp/lifecycle/cf-pcap` and can be executed by any regular
user. Invoking the binary will cause it to emit raw pcap on stdout which can
mess up your terminal so make sure to redirect it into a file. The accepted
parameters are:

* `-interface`: the network interface on which to capture (usually `lo` or
  `eth0`).
* `-snaplen`: how many bytes to capture of each packet.
* `-filter`: pcap-filter to specify which packets to capture.
* `-v`: more verbose output.

## Notes in network traffic in app containers

When envoy is enabled, all traffic passes through both interfaces in the app
container. First, envoy receives it on `eth0` and terminates TLS, then it
forwards the traffic to the app via the `lo` device in plain text. For SSH
traffic the same thing happens, but envoy won't terminate the SSH encryption.

To avoid capturing the SSH traffic which contains the just captured bytes, thus
indefinitely capturing the same content over and over again, cf-pcap will try to
determine the correct port to filter out. This is done on a best-effort basis
and failure to determine the correct filter will not stop the capture from
happening.

## Capturing traffic locally

SSH into the app instance you would like to capture traffic from. To capture the
plain-text HTTP traffic for an app listening on port 8080 you specify the
loopback interface and filter for TCP traffic on port 8080. You can then either
scp the file back to your local machine and use Wireshark to inspect it or show
the packet data with `tcpdump -r` which is also available inside the app
container:

```
$ cf ssh $app
vcap@app:~$ /tmp/lifecycle/cf-pcap -interface lo -filter 'port 8080' -v > capture.pcap
time=2026-02-12T16:58:33.407Z level=DEBUG msg="parsed flags" interface=lo snaplen=65535 filter="port 8080"
time=2026-02-12T16:58:33.428Z level=INFO msg="adjusted filter to avoid capturing SSH session" filter="(not tcp port 2222) and (port 8080)"
^Ctime=2026-02-12T16:59:22.029Z level=INFO msg="received signal, stopping capture" signal=interrupt
time=2026-02-12T16:59:27.456Z level=INFO msg="stopped capture"
vcap@d4a6753d-7a12-4492-48af-9202:~$ tcpdump -r capture.pcap | head -n2
reading from file capture.pcap, link-type EN10MB (Ethernet), snapshot length 65535
16:58:57.377057 IP d4a6753d-7a12-4492-48af-9202.45604 > d4a6753d-7a12-4492-48af-9202.http-alt: Flags [S], seq 3934928449, win 65495, options [mss 65495,sackOK,TS val 3287968384 ecr 0,nop,wscale 7], length 0
16:58:57.377076 IP d4a6753d-7a12-4492-48af-9202.http-alt > d4a6753d-7a12-4492-48af-9202.45604: Flags [S.], seq 708940477, ack 3934928450, win 65483, options [mss 65495,sackOK,TS val 3287968384 ecr 3287968384,nop,wscale 7], length 0

```

## Capturing traffic remotely

Similar to the local capture, you start cf-pcap with the details on what you
want to capture but this time wrap it in the `-c` of cf-ssh. This will send the
pcap output to stdout and you can redirect the content to a local file.
Afterwards, you can again use tcpdumps `-r` to inspect the content:

```
cf ssh $app -c "/tmp/lifecycle/cf-pcap -interface lo -filter 'tcp port 8080'" > capture.pcap
...
^C
...
$ tcpdump -Xr capture.pcap
```
