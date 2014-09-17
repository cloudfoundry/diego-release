#!/usr/bin/env ruby

require "yaml"

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

threads = []

packages.each do |bosh_package, go_package|
  threads << Thread.new do
    spec_path = File.join("packages", bosh_package, "spec")

    spec = YAML.load_file(spec_path)

    # Remove existing ".go" files from the package spec
    spec["files"].reject! { |g| g =~ /\.go$/ }

    # Find all go dependencies of the package
    deps = %x(go list -f '{{join .Deps "\\n"}}' #{go_package}/... | xargs go list -f '{{if not .Standard}}{{.ImportPath}}{{end}}').split

    # Include the package source itself in the spec file
    spec["files"] << "#{go_package}/**/*.go"

    # Add all the dependencies to the spec file
    deps.each do |dep_package|
      spec["files"] << "#{dep_package}/*.go"
    end

    spec["files"].sort!
    File.open(spec_path, "w") do |io|
      YAML.dump(spec, io)
    end

    # check if spec was modified
    if `git status --porcelain -- #{spec_path}`[1] == "M"
      puts spec_path
    end
  end
end

threads.each(&:join)
