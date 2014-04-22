# Each Warden container is a /30 in Warden's network range, which is
# configured as 10.244.0.0/22. There are 256 available entries.
#
# We want two subnets, so I've arbitrarily divided this in half for each.
#
# diego1 will be 10.244.0.0/23
# diego2 will be 10.244.2.0/23
#
# Each network will have 128 subnets, and the first half of each subnet will
# be given static IPs.

require "yaml"
require "netaddr"

diego1_subnets = []
diego1_start = NetAddr::CIDR.create("10.244.4.0/30")

diego2_subnets = []
diego2_start = NetAddr::CIDR.create("10.244.6.0/30")

128.times do
  diego1_subnets << diego1_start
  diego1_start = NetAddr::CIDR.create(diego1_start.next_subnet)

  diego2_subnets << diego2_start
  diego2_start = NetAddr::CIDR.create(diego2_start.next_subnet)
end

puts YAML.dump(
  "networks" => [
    { "name" => "diego1",
      "subnets" => diego1_subnets.collect.with_index do |subnet, idx|
        { "cloud_properties" => {
            "name" => "random",
          },
          "range" => subnet.to_s,
          "reserved" => [subnet[1].ip],
          "static" => idx < 64 ? [subnet[2].ip] : [],
        }
      end
    },
    { "name" => "diego2",
      "subnets" => diego2_subnets.collect.with_index do |subnet, idx|
        { "cloud_properties" => {
            "name" => "random",
          },
          "range" => subnet.to_s,
          "reserved" => [subnet[1].ip],
          "static" => idx < 64 ? [subnet[2].ip] : [],
        }
      end
    },
  ])
