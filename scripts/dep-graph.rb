#!/usr/bin/env ruby

require 'json'
require 'ruby-graphviz'

SRC = File.expand_path("../../src", __FILE__)

COLORS = %w(
  red
  red4
  orange
  orange3
  green2
  green4
  blue
  blue4
  purple
  purple3
  tan4
)

def get_pkg_name(string)
  match = string.match(%r(github.com/cloudfoundry-incubator/([^.]+)))
  match[1].strip if match
end

def subpackage?(child, parent)
  child.start_with?(parent + "/")
end

def color(name)
  COLORS[name.hash % COLORS.length]
end

def add_subgraph(graph, name)
  graph.add_graph("cluster"+name, {
    :bgcolor => "lightgrey",
    :label => name,
    :color => color(name),
    :fontsize => 24,
  })
end

graph = GraphViz.new(:dependency_graph, {
  :type => :digraph
})

graph[:ranksep] = 4
graph.node[:shape] = "rectangle"
graph.edge[:arrowsize] = 2

Dir.entries(SRC).each do |dir|
  path = "#{SRC}/#{dir}"
  next if Dir.glob("#{path}/*.go").empty?

  Dir.chdir(path) do
    job_name = get_pkg_name(`git config --get remote.origin.url`) || next
    dep_pkg_names = JSON.parse(`go list -json`)["Deps"].map { |n| get_pkg_name(n) }.compact

    subgraph = add_subgraph(graph, job_name)
    subgraph.add_node(job_name, :shape => "point")

    dep_pkg_names.each do |dep_pkg_name|
      unless subpackage?(dep_pkg_name, job_name)
        repo_name, subpackage_name = dep_pkg_name.split('/', 2)
        subpackage_name ||= ""
        dep_subgraph = add_subgraph(graph, repo_name)
        dep_subgraph.add_node(subpackage_name)
        graph.add_edge(job_name, subpackage_name, {
          :color => color(job_name),
        })
      end
    end
  end
end

graph.output(
  :dot => "/tmp/graph.dot",
  :png => "/tmp/graph.png",
)

