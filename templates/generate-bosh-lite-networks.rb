#!/usr/bin/env ruby

require "optparse"
require "yaml"
require "netaddr"


options = {
  networks: {}
}

OptionParser.new do |opts|
  opts.banner = <<TEXT
Usage: #$0 [options]\n
  Example:
    #$0 \\
        -ndiego1 -c10.244.16.0/30 -s128 -i64 \\
        -ndiego2 -c10.244.18.0/30 -s128 -i64 \\
        -ndiego3 -c10.244.20.0/30 -s128 -i64

TEXT


  current_network = nil

  opts.on("-nNAME", "--name=NAME", "network name") do |v|
    current_network = v

    options[:networks][v] = {
      start: nil,
      size: 0,
      static_ips: 0,
    }
  end

  opts.on("-cCIDR", "--cidr=CIDR", "subnet start cidr") do |v|
    options[:networks][current_network][:start] = NetAddr::CIDR.create(v)
  end

  opts.on("-sSIZE", "--size=SIZE", "subnet size") do |v|
    options[:networks][current_network][:size] = v.to_i
  end

  opts.on("-iSTATIC_IPS", "--static-ips=STATIC_IPS", "subnet static ips") do |v|
    options[:networks][current_network][:static_ips] = v.to_i
  end
end.parse!

networks = []

options[:networks].each do |network_name, config|
  subnets = []

  cur = config[:start]
  config[:size].times do
    subnets << cur
    cur = NetAddr::CIDR.create(cur.next_subnet)
  end

  networks.push({
    "name" => network_name,
    "subnets" => subnets.collect.with_index do |subnet, idx|
      {
        "range" => subnet.to_s,
        "reserved" => [subnet[1].ip],
        "static" => idx < config[:static_ips] ? [subnet[2].ip] : [],
        "cloud_properties" => {
          "name" => "random",
        },
      }
    end,
  })
end

# skip leading --- so this can be copied
YAML.dump("networks" => networks).each_line.with_index do |l, i|
  next if i == 0
  print l
end
