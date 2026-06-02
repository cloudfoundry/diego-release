# frozen_string_literal: true

require 'rspec'
require 'json'
require 'bosh/template/test'

describe 'locket' do
  let(:release_path) { File.join(File.dirname(__FILE__), '..') }
  let(:release) { Bosh::Template::Test::ReleaseDir.new(release_path) }
  let(:job) { release.job('locket') }

  describe 'locket.json.erb' do
    let(:deployment_manifest_fragment) do
      {
        'diego' => {
          'locket' => {
            'listen_addr' => '0.0.0.0:8891',
            'debug_addr' => '127.0.0.1:17018',
            'log_level' => 'info',
            'sql' => {
              'db_host' => 'sql-db.service.cf.internal',
              'db_port' => 5432,
              'db_schema' => 'locket',
              'db_username' => 'locket',
              'db_password' => 'locket_pw',
              'db_driver' => 'postgres',
              'require_ssl' => false
            }
          }
        },
        'database' => {
          'max_open_connections' => 200,
          'tls' => {
            'enable_identity_verification' => true
          }
        },
        'tls' => {
          'ca_cert' => 'CA CERT',
          'cert' => 'SERVER CERT',
          'key' => 'SERVER KEY'
        },
        'loggregator' => {
          'v2_api_port' => 3458,
          'ca_cert' => 'LOGGREGATOR CA',
          'cert' => 'LOGGREGATOR CERT',
          'key' => 'LOGGREGATOR KEY'
        },
        'set_kernel_parameters' => true
      }
    end

    let(:template) { job.template('config/locket.json') }
    let(:rendered_template) { template.render(deployment_manifest_fragment) }

    describe 'Database timeout configurations' do
      it 'includes db_operation_timeout when specified' do
        deployment_manifest_fragment['diego']['locket']['sql']['db_operation_timeout'] = '15'
        rendered_template_json = JSON.parse(rendered_template)
        expect(rendered_template_json['db_operation_timeout']).to eq('15s')
      end

      it 'uses default db_operation_timeout of 10s when not overridden' do
        rendered_template_json = JSON.parse(rendered_template)
        expect(rendered_template_json['db_operation_timeout']).to eq('10s')
      end

      it 'includes db_connection_timeout when specified' do
        deployment_manifest_fragment['diego']['locket']['sql']['db_connection_timeout'] = '30'
        rendered_template_json = JSON.parse(rendered_template)
        expect(rendered_template_json['db_connection_timeout']).to eq('30s')
      end

      it 'includes db_read_timeout when specified' do
        deployment_manifest_fragment['diego']['locket']['sql']['db_read_timeout'] = '60'
        rendered_template_json = JSON.parse(rendered_template)
        expect(rendered_template_json['db_read_timeout']).to eq('60s')
      end

      it 'includes db_write_timeout when specified' do
        deployment_manifest_fragment['diego']['locket']['sql']['db_write_timeout'] = '60'
        rendered_template_json = JSON.parse(rendered_template)
        expect(rendered_template_json['db_write_timeout']).to eq('60s')
      end
    end

    describe 'basic configuration' do
      it 'renders a valid JSON config' do
        rendered_template_json = JSON.parse(rendered_template)
        expect(rendered_template_json['listen_address']).to eq('0.0.0.0:8891')
        expect(rendered_template_json['database_driver']).to eq('postgres')
        expect(rendered_template_json['max_open_database_connections']).to eq(200)
      end
    end
  end
end
