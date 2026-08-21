# frozen_string_literal: true

# rubocop: disable Metrics/BlockLength
require 'rspec'
require 'json'
require 'ipaddr'
require 'bosh/template/test'

describe 'auctioneer' do
  let(:release_path) { File.join(File.dirname(__FILE__), '..') }
  let(:release) { Bosh::Template::Test::ReleaseDir.new(release_path) }
  let(:job) { release.job('auctioneer') }

  describe 'auctioneer.json.erb' do
    let(:deployment_manifest_fragment) do
      {
        'bpm' => {
          'enabled' => 'true'
        },
        'diego' => {
          'auctioneer' => {
            'bbs' => {
              'ca_cert' => 'CA CERTS',
              'client_cert' => 'CLIENT CERT',
              'client_key' => 'CLIENT KEY'
            },
            'bin_pack_first_fit_weight' => 0,
            'ca_cert' => 'CA CERT',
            'locket' => {
              'client_keepalive_time' => 10,
              'client_keepalive_timeout' => 22
            },
            'rep' => {
              'ca_cert' => 'CA CERT',
              'client_cert' => 'CLIENT CERT',
              'client_key' => 'CLIENT KEY',
              'require_tls' => 'true'
            },
            'server_cert' => 'SERVER CERT',
            'server_key' => 'SERVER KEY',
            'skip_consul_lock' => 'true'
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

    let(:template) { job.template('config/auctioneer.json') }
    let(:rendered_template) { template.render(deployment_manifest_fragment) }

    context 'check if locket keepalive time is bigger than the timeout' do
      it 'fails if the keepalive time is bigger than timeout' do
        deployment_manifest_fragment['diego']['auctioneer']['locket']['client_keepalive_time'] = 23
        expect do
          rendered_template
        end.to raise_error(/The locket client keepalive time property should not be larger than the timeout/)
      end
    end

    context 'BBS health check (CFAR-1457)' do
      let(:rendered_config) { JSON.parse(rendered_template) }

      context 'when enable_bbs_health_check is not set' do
        it 'defaults to disabled and omits the health-check config' do
          expect(rendered_config).not_to have_key('enable_bbs_health_check')
          expect(rendered_config).not_to have_key('bbs_health_check_interval')
          expect(rendered_config).not_to have_key('bbs_health_check_timeout')
          expect(rendered_config).not_to have_key('bbs_health_check_failure_threshold')
        end
      end

      context 'when enable_bbs_health_check is false' do
        before do
          deployment_manifest_fragment['diego']['auctioneer']['enable_bbs_health_check'] = false
        end

        it 'omits the health-check config' do
          expect(rendered_config).not_to have_key('enable_bbs_health_check')
          expect(rendered_config).not_to have_key('bbs_health_check_interval')
          expect(rendered_config).not_to have_key('bbs_health_check_timeout')
          expect(rendered_config).not_to have_key('bbs_health_check_failure_threshold')
        end
      end

      context 'when enable_bbs_health_check is true' do
        before do
          deployment_manifest_fragment['diego']['auctioneer']['enable_bbs_health_check'] = true
        end

        it 'renders the health-check config with default probe settings' do
          expect(rendered_config['enable_bbs_health_check']).to eq(true)
          expect(rendered_config['bbs_health_check_interval']).to eq('10s')
          expect(rendered_config['bbs_health_check_timeout']).to eq('5s')
          expect(rendered_config['bbs_health_check_failure_threshold']).to eq(3)
        end

        context 'with overridden probe settings' do
          before do
            deployment_manifest_fragment['diego']['auctioneer']['bbs_health_check_interval'] = '30s'
            deployment_manifest_fragment['diego']['auctioneer']['bbs_health_check_timeout'] = '15s'
            deployment_manifest_fragment['diego']['auctioneer']['bbs_health_check_failure_threshold'] = 5
          end

          it 'renders the overridden values' do
            expect(rendered_config['enable_bbs_health_check']).to eq(true)
            expect(rendered_config['bbs_health_check_interval']).to eq('30s')
            expect(rendered_config['bbs_health_check_timeout']).to eq('15s')
            expect(rendered_config['bbs_health_check_failure_threshold']).to eq(5)
          end
        end
      end
    end
  end
end
