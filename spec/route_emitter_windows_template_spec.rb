# frozen_string_literal: true

# frozen_string_literal: true

# rubocop: disable Metrics/BlockLength
require 'rspec'
require 'json'
require 'bosh/template/test'

describe 'route_emitter' do
  let(:release_path) { File.join(File.dirname(__FILE__), '..') }
  let(:release) { Bosh::Template::Test::ReleaseDir.new(release_path) }
  let(:job) { release.job('route_emitter_windows') }

  describe 'route_emitter.json.erb' do
    let(:deployment_manifest_fragment) do
      {
        'bpm' => {
          'enabled' => 'true'
        },
        'diego' => {
          'route_emitter' => {
            'bbs' => {
              'ca_cert' => 'CA CERTS',
              'client_cert' => 'CLIENT CERT',
              'client_key' => 'CLIENT KEY'
            },
            'nats' => {
              'ca_cert' => 'CA CERTS',
              'client_cert' => 'CLIENT CERT',
              'client_key' => 'CLIENT KEY',
              'machines' => {
                'nats_addresses' => '1.2.3.4'
              }
            }
          }
        },
        'enable_consul_service_registration' => 'false',
        'loggregator' => 'LOGGREGATOR PROPS',
        'logging' => {
          'format' => {
            'timestamp' => 'rfc3339'
          }
        }
      }
    end

    let(:template) { job.template('config/route_emitter.json') }
    let(:rendered_template) { template.render(deployment_manifest_fragment) }

    context 'Check jitter_factor default value' do
      it 'Defaults to 0.2 if no value is provided' do
        json_data = JSON.parse(rendered_template)
        jitter_value = json_data['jitter_factor']
        expect(jitter_value).to eq(0.2)
      end
    end

    context 'Check if jitter_factor value is less than 1.0' do
      it 'fails if jitter_factor value is greater than 1.0' do
        deployment_manifest_fragment['diego']['route_emitter']['jitter_factor'] = 1.1
        expect do
          rendered_template
        end.to raise_error(/diego.route_emitter.jitter_factor must be a float between 0.0 and 1.0/)
      end
    end

    context 'Check if jitter_factor value is greater than 0.0' do
      it 'fails if jitter_factor value is less than 0.0' do
        deployment_manifest_fragment['diego']['route_emitter']['jitter_factor'] = -0.1
        expect do
          rendered_template
        end.to raise_error(/diego.route_emitter.jitter_factor must be a float between 0.0 and 1.0/)
      end
    end

    context 'Check if jitter_factor value type is Float' do
      it 'fails if jitter_factor value type is not Float' do
        deployment_manifest_fragment['diego']['route_emitter']['jitter_factor'] = 1
        expect do
          rendered_template
        end.to raise_error(/diego.route_emitter.jitter_factor must be a float between 0.0 and 1.0/)
      end
    end
  end
end
