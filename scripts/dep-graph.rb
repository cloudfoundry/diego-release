#!/usr/bin/env ruby

require 'yajl'
require 'ruby-graphviz'

def main
  graph = GraphViz.new(:dependency_graph, {
    :type => :digraph
  })

  graph[:ranksep] = 4
  graph.node[:shape] = "rectangle"
  graph.edge[:arrowsize] = 2

  Dir.entries(src_dir).each do |dir|
    path = "#{src_dir}/#{dir}"
    next if [".", ".."].include?(dir)
    next if Dir.glob("#{path}/**/*.go").empty?

    Dir.chdir(path) do
      job_name = get_pkg_name(`git config --get remote.origin.url`) || next

      dep_pkg_names = []
      Yajl::Parser.parse(`go list -json ./...`) do |pkg|
        dep_pkg_names.concat(pkg["Deps"].map { |n| get_pkg_name(n) }.compact)
      end
      dep_pkg_names.uniq!

      subgraph = add_subgraph(graph, job_name)
      add_node(subgraph, job_name, nil)

      dep_pkg_names.each do |dep_pkg_name|
        unless subpackage?(dep_pkg_name, job_name)
          repo_name, subpackage_name = dep_pkg_name.split('/', 2)

          dep_subgraph = add_subgraph(graph, repo_name)
          add_node(dep_subgraph, dep_pkg_name, subpackage_name)
          add_edge(graph, job_name, dep_pkg_name)
        end
      end
    end
  end

  path = "/tmp/graph.png"
  graph.output(:png => path)
  puts "Wrote graph to `#{path}`"
end

def src_dir
  File.expand_path("../../src", __FILE__)
end

def get_pkg_name(string)
  match = string.match(%r(github.com/cloudfoundry-incubator/([^.]+)))
  match[1].strip if match
end

def subpackage?(child, parent)
  child.start_with?(parent + "/")
end

def color(name)
  @colors ||= %w(
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
  @colors_by_name ||= {}
  @colors_by_name[name] ||= @colors[@colors_by_name.size % @colors.length]
end

def add_subgraph(graph, name)
  graph_name = "cluster" + name
  graph.get_graph(graph_name) || graph.add_graph(graph_name, {
    :bgcolor => "lightgrey",
    :label => name,
    :color => color(name),
    :fontsize => 24,
  })
end

def add_node(graph, name, label)
  graph.add_node(node_id(name), {
    :label => label || "",
    :shape => label ? "rectangle" : "point"
  })
end

def add_edge(graph, from_name, to_name)
  graph.add_edge(node_id(from_name), node_id(to_name), {
    :color => color(from_name),
  })
end

def node_id(pkg_name)
  @node_ids ||= {}
  @node_ids[pkg_name] ||= @node_ids.size.to_s
end

main
