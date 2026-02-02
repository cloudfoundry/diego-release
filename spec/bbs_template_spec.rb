# frozen_string_literal: true

# rubocop: disable Metrics/BlockLength
require 'rspec'
require 'json'
require 'bosh/template/test'

describe 'bbs' do
  let(:release_path) { File.join(File.dirname(__FILE__), '..') }
  let(:release) { Bosh::Template::Test::ReleaseDir.new(release_path) }
  let(:job) { release.job('bbs') }

  describe 'bbs.json.erb' do
    let(:deployment_manifest_fragment) do
      {
        'bpm' => {
          'enabled' => 'true'
        },
        'diego' => {
          'bbs' => {
            'active_key_label' => 'ACTIVE KEY',
            'detect_consul_cell_registrations' => 'false',
            'encryption_keys' => [
              'label' => 'KEY LABEL',
              'passphrase' => 'PASSPHRASE'
            ],
            'sql' => {
              'db_host' => 'sql-db.service.cf.internal',
              'db_port' => 3306,
              'db_schema' => 'diego',
              'db_username' => 'diego',
              'db_password' => 'DB PASSWORD',
              'db_driver' => 'mysql',
              'ca_cert' => 'CA CERT',
              'require_ssl' => true
            },
            'ca_cert' => 'CA CERT',
            'auctioneer' => {
              'ca_cert' => 'CA CERT',
              'client_cert' => 'CLIENT CERT',
              'client_key' => 'CLIENT KEY'
            },
            'locket' => {
              'client_keepalive_time' => 10,
              'client_keepalive_timeout' => 22
            },
            'server_cert' => 'SERVER CERT',
            'server_key' => 'SERVER KEY',
            'skip_consul_lock' => 'true',
            'rep' => {
              'require_tls' => 'true',
              'ca_cert' => 'CA CERT',
              'client_cert' => 'CLIENT CERT',
              'client_key' => 'CLIENT KEY'
            },
            'metrics' => {
              'advanced_metrics' => {
                'enabled' => true,
                'route_config' => {
                  'request_count' => ['StartActualLRP'],
                  'request_latency' => []
                }
              }
            }
          },
          'enable_consul_service_registration' => 'false',
          'loggregator' => {
            'ca_cert' => 'CA CERT',
            'client_cert' => 'CLIENT CERT',
            'client_key' => 'CLIENT KEY'
          },
          'logging' => {
            'format' => {
              'timestamp' => 'rfc3339'
            }
          }
        }
      }
    end

    let(:deployment_manifest_fragment_default) { deployment_manifest_fragment.dup }
    let(:template) { job.template('config/bbs.json') }
    let(:rendered_template) { template.render(deployment_manifest_fragment) }

    context 'check if locket keepalive time is bigger than the timeout' do
      it 'fails if the keepalive time is bigger than timeout' do
        deployment_manifest_fragment['diego']['bbs']['locket']['client_keepalive_time'] = 23
        expect do
          rendered_template
        end.to raise_error(/The locket client keepalive time property should not be larger than the timeout/)
      end
    end

    describe 'Database timeout configurations' do
      it 'includes db timeout settings when specified' do
        deployment_manifest_fragment['diego']['bbs']['sql']['db_connection_timeout'] = '30'
        deployment_manifest_fragment['diego']['bbs']['sql']['db_read_timeout'] = '30'
        deployment_manifest_fragment['diego']['bbs']['sql']['db_write_timeout'] = '45'
        rendered_template_json = JSON.parse(rendered_template)
        expect(rendered_template_json['db_connection_timeout']).to eq('30s')
        expect(rendered_template_json['db_read_timeout']).to eq('30s')
        expect(rendered_template_json['db_write_timeout']).to eq('45s')
      end

      it 'uses default timeout values when not specified' do
        rendered_template_json = JSON.parse(rendered_template)
        expect(rendered_template_json['db_connection_timeout']).to eq('30s')
        expect(rendered_template_json['db_read_timeout']).to eq('60s')
        expect(rendered_template_json['db_write_timeout']).to eq('60s')
      end
    end

    describe 'logging' do
      context 'log level' do
        it 'sets the default log level' do
          rendered_template_json = JSON.parse(rendered_template)
          expect(rendered_template_json['log_level']).to eq('info')
        end

        context 'when log level is specified' do
          before do
            deployment_manifest_fragment['diego']['bbs']['log_level']= 'debug'
          end
          it do
          rendered_template_json = JSON.parse(rendered_template)
          expect(rendered_template_json['log_level']).to eq('debug')
          end
        end
      end

      context 'debug_lrp_start_heartbeats' do
        it 'sets the debug_lrp_start_heartbeats off by default' do
          rendered_template_json = JSON.parse(rendered_template)
          expect(rendered_template_json['debug_lrp_start_heartbeats']).to be false
        end

        context 'when log level is specified' do
          before do
            deployment_manifest_fragment['diego']['bbs']['debug_lrp_start_heartbeats'] = true
          end
          it do
          rendered_template_json = JSON.parse(rendered_template)
          expect(rendered_template_json['debug_lrp_start_heartbeats']).to be true
          end
        end
      end
    end


    describe 'database connection lifetime' do 
      it 'sets the db connection lifetime when specified' do
        deployment_manifest_fragment['diego']['bbs']['sql']['max_connection_lifetime'] = '10'
        rendered_template_json = JSON.parse(rendered_template)
        expect(rendered_template_json['max_database_connection_lifetime']).to eq('10s')
      end
      it 'sets the db connection lifetime when specified' do
        rendered_template_json = JSON.parse(rendered_template)
        expect(rendered_template_json['max_database_connection_lifetime']).to eq('90s')
      end
    end

    describe 'Advanced Metrics' do
      it 'check if bbs.json doesn\'t include \'advanced_metrics\' on \'enabled\' value equal to false' do
        # if diego.bbs.metrics.advanced_metrics misses diego.bbs.metrics.advanced_metrics.enabled defaults to false
        deployment_manifest_fragment['diego']['bbs']['metrics'] = nil
        expect(rendered_template).not_to include('advanced_metrics')
      end

      it 'check if bbs.json includes \'advanced_metrics\' on \'enabled\' value equal to true' do
        # if diego.bbs.metrics.advanced_metrics misses diego.bbs.metrics.advanced_metrics.enabled defaults to false
        # test data already has diego.bbs.metrics.advanced_metrics.enabled set to true
        rendered_template_json = JSON.parse(rendered_template)

        expect(rendered_template_json['advanced_metrics']).not_to eq(nil)
        expect(rendered_template_json['advanced_metrics']['enabled']).to eq(true)
        expect(rendered_template_json['advanced_metrics']['route_config']).not_to eq(nil)
        expect(rendered_template_json['advanced_metrics']['route_config']['request_count']).to eq(['StartActualLRP'])
        expect(rendered_template_json['advanced_metrics']['route_config']['request_latency']).to eq([])
      end

      it 'fails if the diego.bbs.metrics.advanced_metrics.enabled is not a boolean' do
        deployment_manifest_fragment['diego']['bbs']['metrics']['advanced_metrics']['enabled'] = 'true'

        expect do
          rendered_template
        end.to raise_error('diego.bbs.metrics.advanced_metrics.enabled should be a boolean')
      end

      it 'fails if the diego.bbs.metrics.advanced_metrics.route_config.request_count is not an array' do
        deployment_manifest_fragment['diego']['bbs']['metrics']['advanced_metrics']['route_config']['request_count'] =
          'true'

        expect do
          rendered_template
        end.to raise_error('diego.bbs.metrics.advanced_metrics.route_config.request_count should be an array')
      end

      it 'fails if the diego.bbs.metrics.advanced_metrics.route_config.request_latency is not an array' do
        deployment_manifest_fragment['diego']['bbs']['metrics']['advanced_metrics']['route_config']['request_count'] =
          'true'

        expect do
          rendered_template
        end.to raise_error('diego.bbs.metrics.advanced_metrics.route_config.request_count should be an array')
      end

      it 'fails if request_count and request_latency are both empty' do
        deployment_manifest_fragment['diego']['bbs']['metrics']['advanced_metrics']['route_config']['request_count'] = \
          []

        expect do
          rendered_template
        end.to raise_error('diego.bbs.metrics.advanced_metrics.route_config.request_count and ' \
                           'diego.bbs.metrics.advanced_metrics.route_config.request_latency cannot both be empty.')
      end

      it 'succeeds if at least one of request_count or request_latency is not empty' do
        rendered_template
      end
    end
  end
end
