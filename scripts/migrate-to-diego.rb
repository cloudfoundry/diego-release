#!/usr/bin/env ruby

require 'json'
require 'shellwords'

HELP = <<EOF
DEA to Diego Migrator

Move a batch of applications from DEA to Diego, using the CF CLI. The next batch
of applications that has diego=false has diego-enabled called on it. Simple as
that.

This tool requires `cf` and `parallel` to be in your PATH.

    Usage:
    migrate-to-diego BATCH_SIZE [MAX_IN_FLIGHT] [REVERSE]

    BATCH_SIZE - the number of applications
    MAX_IN_FLIGHT - the number of applications migrated in parallel (default to 1)
    REVERSE - migrates from diego back to the DEA (default to false)
EOF

def main
  batch_size, max_in_flight, reverse = initialize_args
  if reverse
    puts "Migrating in reverse - Diego back to DEA"
  end
  queue = Queue.new
  apps_running_on_dea(batch_size,reverse).each {|guid| queue << guid}
  migrate_in_parallel(queue,max_in_flight,reverse)
end

def initialize_args
  begin
    batch_size = Integer(ARGV[0])
    if ARGV[1].nil?
      max_in_flight = 1
    else
      max_in_flight = Integer(ARGV[1])
    end
    case ARGV[2]
    when "true"
      reverse = true
    when "false"
      reverse = false
    when nil
      reverse = false
    else
      raise "REVERSE must be boolean"
    end
  rescue
    puts "ERROR: BATCH_SIZE and MAX_IN_FLIGHT must be a valid integer, REVERSE must be boolean"
    abort HELP
  end

  cf_cmd = `which cf`
  if cf_cmd.empty?
    puts "ERROR: cf not found in path. Please download the CLI that matches your CLoud Foundry installation."
    abort HELP
  end

  return batch_size, max_in_flight, reverse
end

def next_page(page_url)
  cf_output = `cf curl #{page_url.shellescape}`
  output=JSON.parse(cf_output)
  next_url = output['next_url']
  app_guids = output['resources'].collect { |r| r['metadata']['guid'] }
  return next_url, app_guids
end

def apps_running_on_dea(batch_number,reverse)
  dea_apps_guid = []
  unfinished_number = batch_number
  # Iterate 100 apps a time, which is the max value for cc api
  page_url = "/v2/apps?q=diego:#{reverse.to_s}&results-per-page=100"
  while unfinished_number > 0 && (!page_url.nil?)
    page_url, app_guids = next_page(page_url)
    dea_apps_guid += app_guids
    unfinished_number -=100
  end
  dea_apps_guid[0..batch_number-1]
end


def migrate_app(app_guid, reverse)
  puts "migrating #{app_guid}"
  cf_output= `cf curl /v2/apps/#{app_guid} -X PUT -d '{"diego":#{(!reverse).to_s}}'`
  output = JSON.parse(cf_output)
  unless output["error_code"].nil?
    puts "ERROR: Failed to set diego to true for app #{app_guid}\n#{cf_output}"
    return false
  end
  puts "completed #{app_guid}"
  true
end

def migrate_in_parallel(queue, max_in_flight, reverse)
  thread_pool = []
  max_in_flight.times do |i|
    thread_pool << Thread.new do
      begin
        while guid = queue.pop(true)
          migrate_app(guid,reverse)
        end
      rescue ThreadError
      end
    end
  end
  thread_pool.each {|t| t.join}
end


main
