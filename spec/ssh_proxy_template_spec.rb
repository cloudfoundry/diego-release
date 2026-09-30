# frozen_string_literal: true

# rubocop: disable Metrics/BlockLength
require 'rspec'
require 'json'
require 'ipaddr'
require 'bosh/template/test'

describe 'ssh_proxy' do
  let(:release_path) { File.join(File.dirname(__FILE__), '..') }
  let(:release) { Bosh::Template::Test::ReleaseDir.new(release_path) }
  let(:job) { release.job('ssh_proxy') }
  let(:deployment_manifest_fragment) do
    {
      'diego' => {
        'ssh_proxy' => {
          'host_key' => 'HOST KEY',
          'bbs' => {
            'ca_cert' => 'BBS CA CERT',
            'client_cert' => 'BBS CLIENT CERT',
            'client_key' => 'BBS CLIENT KEY'
          }
        }
      },
      'loggregator' => {
        'ca_cert' => 'LOGGREGATOR CA CERT',
        'cert' => 'LOGGREGATOR CERT',
        'key' => 'LOGGREGATOR KEY'
      }
    }
  end

  describe 'ssh_proxy.json.erb' do
    let(:template) { job.template('config/ssh_proxy.json') }
    let(:rendered_config) { JSON.parse(template.render(deployment_manifest_fragment)) }

    context 'when max_connection_duration_in_seconds is empty' do
      it 'omits the connection duration to allow unlimited sessions' do
        expect(rendered_config).not_to have_key('max_connection_duration')
      end
    end

    context 'when max_connection_duration_in_seconds is configured' do
      before do
        deployment_manifest_fragment['diego']['ssh_proxy']['max_connection_duration_in_seconds'] = 86_400
      end

      it 'renders the duration in seconds' do
        expect(rendered_config['max_connection_duration']).to eq('86400s')
      end
    end

    context 'when max_connection_duration_in_seconds is zero' do
      before do
        deployment_manifest_fragment['diego']['ssh_proxy']['max_connection_duration_in_seconds'] = 0
      end

      it 'fails rendering because finite durations must be positive' do
        expect { rendered_config }.to raise_error(/must be a positive integer/)
      end
    end

    context 'when max_connection_duration_in_seconds is not an integer' do
      before do
        deployment_manifest_fragment['diego']['ssh_proxy']['max_connection_duration_in_seconds'] = 1.5
      end

      it 'fails rendering' do
        expect { rendered_config }.to raise_error(/must be a positive integer/)
      end
    end
  end
end
# rubocop: enable Metrics/BlockLength
