# frozen_string_literal: true

# rubocop: disable Metrics/BlockLength
require 'rspec'
require 'json'
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

    context 'when max_connection_duration_in_seconds is not configured' do
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

      it 'omits the connection duration to allow unlimited sessions' do
        expect(rendered_config).not_to have_key('max_connection_duration')
      end
    end

    [-1, 1.5, '', '3600', false].each do |value|
      context "when max_connection_duration_in_seconds is #{value.inspect}" do
        before do
          deployment_manifest_fragment['diego']['ssh_proxy']['max_connection_duration_in_seconds'] = value
        end

        it 'rejects values that are not non-negative integers' do
          expect { rendered_config }.to raise_error(/must be a non-negative integer/)
        end
      end
    end

    describe 'cloud controller access' do
      context 'when no cc properties or links are configured' do
        it 'defaults to the plain HTTP external endpoint' do
          expect(rendered_config['cc_api_url']).to eq('http://cloud-controller-ng.service.cf.internal:9022')
        end

        it 'does not configure any cc TLS certificates' do
          expect(rendered_config['cc_api_ca_cert']).to be_nil
          expect(rendered_config['cc_api_client_cert']).to be_nil
          expect(rendered_config['cc_api_client_key']).to be_nil
        end
      end

      context 'when the cloud_controller_https_endpoint link provides public TLS' do
        let(:cc_link) do
          Bosh::Template::Test::Link.new(
            name: 'cloud_controller_https_endpoint',
            properties: {
              'cc' => {
                'internal_service_hostname' => 'cloud-controller-ng.service.cf.internal',
                'public_tls' => {
                  'port' => 9024,
                  'ca_cert' => 'CC PUBLIC TLS CA CERT'
                }
              }
            }
          )
        end
        let(:rendered_config) do
          JSON.parse(template.render(deployment_manifest_fragment, consumes: [cc_link]))
        end

        it 'uses the public TLS endpoint with OAuth (server-auth only)' do
          expect(rendered_config['cc_api_url']).to eq('https://cloud-controller-ng.service.cf.internal:9024')
          expect(rendered_config['cc_api_ca_cert']).to eq('/var/vcap/jobs/ssh_proxy/config/certs/cc/cc_api_ca_cert.crt')
          expect(rendered_config['cc_api_client_cert']).to be_nil
          expect(rendered_config['cc_api_client_key']).to be_nil
        end
      end

      context 'when the cc mutual TLS properties are configured' do
        before do
          deployment_manifest_fragment['diego']['ssh_proxy']['cc'] = {
            'tls_port' => 9023,
            'ca_cert' => 'CC MUTUAL TLS CA CERT',
            'client_cert' => 'CC CLIENT CERT',
            'client_key' => 'CC CLIENT KEY'
          }
        end

        it 'targets the mutual TLS listener and presents a client certificate' do
          expect(rendered_config['cc_api_url']).to eq('https://cloud-controller-ng.service.cf.internal:9023')
          expect(rendered_config['cc_api_ca_cert']).to eq('/var/vcap/jobs/ssh_proxy/config/certs/cc/cc_api_ca_cert.crt')
          expect(rendered_config['cc_api_client_cert']).to eq('/var/vcap/jobs/ssh_proxy/config/certs/cc/client.crt')
          expect(rendered_config['cc_api_client_key']).to eq('/var/vcap/jobs/ssh_proxy/config/certs/cc/client.key')
        end

        context 'and the public TLS link is also present' do
          let(:cc_link) do
            Bosh::Template::Test::Link.new(
              name: 'cloud_controller_https_endpoint',
              properties: {
                'cc' => {
                  'internal_service_hostname' => 'cloud-controller-ng.service.cf.internal',
                  'public_tls' => {
                    'port' => 9024,
                    'ca_cert' => 'CC PUBLIC TLS CA CERT'
                  }
                }
              }
            )
          end
          let(:rendered_config) do
            JSON.parse(template.render(deployment_manifest_fragment, consumes: [cc_link]))
          end

          it 'prefers the mutual TLS listener over the public TLS link' do
            expect(rendered_config['cc_api_url']).to eq('https://cloud-controller-ng.service.cf.internal:9023')
            expect(rendered_config['cc_api_client_cert']).to eq('/var/vcap/jobs/ssh_proxy/config/certs/cc/client.crt')
            expect(rendered_config['cc_api_client_key']).to eq('/var/vcap/jobs/ssh_proxy/config/certs/cc/client.key')
          end
        end
      end

      context 'when the cc mutual TLS properties are only partially configured' do
        before do
          deployment_manifest_fragment['diego']['ssh_proxy']['cc'] = {
            'tls_port' => 9023,
            'ca_cert' => 'CC MUTUAL TLS CA CERT'
          }
        end

        it 'does not switch to the mutual TLS listener' do
          expect(rendered_config['cc_api_url']).to eq('http://cloud-controller-ng.service.cf.internal:9022')
          expect(rendered_config['cc_api_client_cert']).to be_nil
          expect(rendered_config['cc_api_client_key']).to be_nil
        end
      end
    end
  end
end
# rubocop: enable Metrics/BlockLength
