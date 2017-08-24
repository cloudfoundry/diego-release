#!/usr/bin/env bash
docker run --rm -v $PWD:/envoy-binary/ lyft/envoy:fc747b3c2fd49b1260484572071fe4194cd6824d /bin/bash -c 'cp `which envoy` /envoy-binary/'
tar -czf envoy-1.3.0.tgz envoy

bosh add-blob envoy-1.3.0.tgz proxy/envoy-1.3.0.tgz
mv envoy-1.3.0.tgz blobs/proxy/envoy-1.3.0.tgz
rm -rf envoy
