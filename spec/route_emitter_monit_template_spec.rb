# frozen_string_literal: true

require 'rspec'
require 'yaml'
require 'bosh/template/test'

# The monit file is not listed under `templates:` in the job spec, so it is rendered
# directly with Bosh::Template::Test::Template (BOSH renders it as ERB as well).
describe 'route_emitter monit' do
  let(:job_path) { File.join(File.dirname(__FILE__), '..', 'jobs', 'route_emitter') }
  let(:spec) { YAML.load_file(File.join(job_path, 'spec')) }
  let(:template) { Bosh::Template::Test::Template.new(spec, File.join(job_path, 'monit')) }
  let(:stop_line) { '  stop program "/var/vcap/jobs/bpm/bin/bpm stop route_emitter"' }

  def rendered_stop_line(rendered)
    rendered.lines.find { |l| l.include?('stop program') }
  end

  it 'keeps the monit default stop timeout when the property is not set' do
    expect(rendered_stop_line(template.render({}))).to eq("#{stop_line}\n")
  end

  it 'sets the stop timeout when the property is set' do
    rendered = template.render({ 'diego' => { 'route_emitter' => { 'monit_stop_timeout_in_seconds' => 120 } } })
    expect(rendered_stop_line(rendered)).to eq("#{stop_line} with timeout 120 seconds\n")
    expect(rendered).to include("  group vcap\n")
  end

  it 'rejects a non-integer value' do
    expect do
      template.render({ 'diego' => { 'route_emitter' => { 'monit_stop_timeout_in_seconds' => 'two minutes' } } })
    end.to raise_error(ArgumentError)
  end
end
