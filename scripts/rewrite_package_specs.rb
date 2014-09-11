#!/usr/bin/env ruby

packages = {
  "auctioneer" => "github.com/cloudfoundry-incubator/auctioneer",
  "converger" => "github.com/cloudfoundry-incubator/converger",
  "docker-circus" => "github.com/cloudfoundry-incubator/docker-circus",
  "etcd" => "github.com/coreos/etcd",
  "etcd_metrics_server" => "github.com/cloudfoundry-incubator/etcd-metrics-server",
  "executor" => "github.com/cloudfoundry-incubator/executor",
  "file_server" => "github.com/cloudfoundry-incubator/file-server",
  "garden-linux" => "github.com/cloudfoundry-incubator/garden-linux",
  "linux-circus" => "github.com/cloudfoundry-incubator/linux-circus",
  "nsync" => "github.com/cloudfoundry-incubator/nsync",
  "rep" => "github.com/cloudfoundry-incubator/rep",
  "route_emitter" => "github.com/cloudfoundry-incubator/route-emitter",
  "runtime_metrics_server" => "github.com/cloudfoundry-incubator/runtime-metrics-server",
  "stager" => "github.com/cloudfoundry-incubator/stager",
  "tps" => "github.com/cloudfoundry-incubator/tps",
}

packages.each do |bosh_package, go_package|
  system("sed -i '' '/\\.go$/d' ./packages/#{bosh_package}/spec") or fail
  deps=%x(go list -f '{{join .Deps "\\n"}}' #{go_package}/... | xargs go list -f '{{if not .Standard}}{{.ImportPath}}{{end}}').split
  system("echo '  - #{go_package}/**/*.go' >> packages/#{bosh_package}/spec") or fail
  deps.each do |dep_package|
    system("echo '  - #{dep_package}/*.go' >> packages/#{bosh_package}/spec") or fail
  end
end
