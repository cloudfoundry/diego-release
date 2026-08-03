# frozen_string_literal: true

# rubocop: disable Metrics/BlockLength
require 'rspec'
require 'json'
require 'bosh/template/test'

describe 'file_server' do
  let(:release_path) { File.join(File.dirname(__FILE__), '..') }
  let(:release) { Bosh::Template::Test::ReleaseDir.new(release_path) }
  let(:job) { release.job('file_server') }

  describe 'file_server.json.erb' do
    let(:deployment_manifest_fragment) do
      {
        'diego' => {
          'file_server' => {
            'listen_addr' => '0.0.0.0:8080',
            'static_directory' => '/var/vcap/jobs/file_server/packages/',
            'log_level' => 'info',
            'debug_addr' => ''
          }
        },
        'https_server_enabled' => false,
        'https_listen_addr' => '0.0.0.0:8443',
        'loggregator' => {
          'v2_api_port' => 3458,
          'ca_cert' => 'LOGGREGATOR CA CERT',
          'cert' => 'LOGGREGATOR CLIENT CERT',
          'key' => 'LOGGREGATOR CLIENT KEY'
        }
      }
    end

    let(:template) { job.template('config/file_server.json') }
    let(:rendered_template) { template.render(deployment_manifest_fragment) }

    describe 'default configuration' do
      it 'renders the config with default values' do
        rendered_template_json = JSON.parse(rendered_template)
        expect(rendered_template_json['server_address']).to eq('0.0.0.0:8080')
        expect(rendered_template_json['debug_address']).to eq('')
        expect(rendered_template_json['static_directory']).to eq('/var/vcap/jobs/file_server/packages/')
        expect(rendered_template_json['https_server_enabled']).to be false
        expect(rendered_template_json['https_listen_addr']).to eq('0.0.0.0:8443')
        expect(rendered_template_json['log_level']).to eq('info')
      end

      it 'uses rfc3339 time format' do
        rendered_template_json = JSON.parse(rendered_template)
        expect(rendered_template_json['time_format']).to eq('rfc3339')
      end

      it 'does not include TLS cert paths when TLS is not configured' do
        rendered_template_json = JSON.parse(rendered_template)
        expect(rendered_template_json['cert_file']).to be_nil
        expect(rendered_template_json['key_file']).to be_nil
        expect(rendered_template_json['client_ca_cert_file']).to be_nil
      end
    end

    describe 'log level configuration' do
      it 'sets the default log level to info' do
        rendered_template_json = JSON.parse(rendered_template)
        expect(rendered_template_json['log_level']).to eq('info')
      end

      it 'allows setting custom log level' do
        deployment_manifest_fragment['diego']['file_server']['log_level'] = 'debug'
        rendered_template_json = JSON.parse(rendered_template)
        expect(rendered_template_json['log_level']).to eq('debug')
      end
    end

    describe 'debug_addr configuration' do
      it 'sets debug_address to empty string by default' do
        rendered_template_json = JSON.parse(rendered_template)
        expect(rendered_template_json['debug_address']).to eq('')
      end

      it 'renders the debug_address when debug_addr is configured' do
        deployment_manifest_fragment['diego']['file_server']['debug_addr'] = '127.0.0.1:17005'
        rendered_template_json = JSON.parse(rendered_template)
        expect(rendered_template_json['debug_address']).to eq('127.0.0.1:17005')
      end
    end

    describe 'IP address validation' do
      it 'validates diego.file_server.listen_addr' do
        deployment_manifest_fragment['diego']['file_server']['listen_addr'] = 'invalid-ip:8080'
        expect do
          rendered_template
        end.to raise_error(/Invalid diego.file_server.listen_addr/)
      end

      it 'validates diego.file_server.debug_addr' do
        deployment_manifest_fragment['diego']['file_server']['debug_addr'] = 'invalid-ip:17005'
        expect do
          rendered_template
        end.to raise_error(/Invalid diego.file_server.debug_addr/)
      end

      it 'validates https_listen_addr' do
        deployment_manifest_fragment['https_listen_addr'] = 'invalid-ip:8443'
        expect do
          rendered_template
        end.to raise_error(/Invalid https_listen_addr/)
      end

      it 'accepts valid IPv4 addresses' do
        deployment_manifest_fragment['diego']['file_server']['listen_addr'] = '10.0.0.1:8080'
        deployment_manifest_fragment['diego']['file_server']['debug_addr'] = '127.0.0.1:17005'
        deployment_manifest_fragment['https_listen_addr'] = '0.0.0.0:8443'
        expect { rendered_template }.not_to raise_error
      end
    end

    describe 'TLS configuration' do
      context 'when TLS cert and key are provided' do
        before do
          deployment_manifest_fragment['tls'] = {
            'cert' => 'TLS CERT',
            'key' => 'TLS KEY'
          }
        end

        it 'includes TLS cert and key file paths' do
          rendered_template_json = JSON.parse(rendered_template)
          expect(rendered_template_json['cert_file']).to eq('/var/vcap/jobs/file_server/config/certs/tls.crt')
          expect(rendered_template_json['key_file']).to eq('/var/vcap/jobs/file_server/config/certs/tls.key')
        end
      end

      context 'when TLS client CA cert is provided' do
        before do
          deployment_manifest_fragment['tls'] = {
            'cert' => 'TLS CERT',
            'key' => 'TLS KEY',
            'client_ca_cert' => 'CLIENT CA CERT'
          }
        end

        it 'includes client CA cert file path' do
          rendered_template_json = JSON.parse(rendered_template)
          expect(rendered_template_json['client_ca_cert_file']).to eq('/var/vcap/jobs/file_server/config/certs/tls.client_ca_cert')
        end
      end

      context 'when HTTPS server is enabled' do
        before do
          deployment_manifest_fragment['https_server_enabled'] = true
        end

        it 'requires both tls.cert and tls.key' do
          expect do
            rendered_template
          end.to raise_error(/tls.cert and tls.key are required if https_server_enabled is set to true/)
        end

        it 'succeeds when both cert and key are provided' do
          deployment_manifest_fragment['tls'] = {
            'cert' => 'TLS CERT',
            'key' => 'TLS KEY'
          }
          expect { rendered_template }.not_to raise_error
        end

        it 'renders HTTPS configuration' do
          deployment_manifest_fragment['tls'] = {
            'cert' => 'TLS CERT',
            'key' => 'TLS KEY'
          }
          rendered_template_json = JSON.parse(rendered_template)
          expect(rendered_template_json['https_server_enabled']).to be true
          expect(rendered_template_json['https_listen_addr']).to eq('0.0.0.0:8443')
        end
      end
    end

    describe 'loggregator configuration' do
      it 'includes loggregator configuration' do
        rendered_template_json = JSON.parse(rendered_template)
        loggregator_config = rendered_template_json['loggregator']

        expect(loggregator_config['loggregator_api_port']).to eq(3458)
        expect(loggregator_config['loggregator_ca_path']).to eq('/var/vcap/jobs/file_server/config/certs/loggregator/ca.crt')
        expect(loggregator_config['loggregator_cert_path']).to eq('/var/vcap/jobs/file_server/config/certs/loggregator/client.crt')
        expect(loggregator_config['loggregator_key_path']).to eq('/var/vcap/jobs/file_server/config/certs/loggregator/client.key')
        expect(loggregator_config['loggregator_job_origin']).to eq('file_server')
        expect(loggregator_config['loggregator_source_id']).to eq('file_server')
      end

      it 'uses custom loggregator port when specified' do
        deployment_manifest_fragment['loggregator']['v2_api_port'] = 4444
        rendered_template_json = JSON.parse(rendered_template)
        expect(rendered_template_json['loggregator']['loggregator_api_port']).to eq(4444)
      end
    end
  end

  describe 'tls.crt.erb' do
    let(:template) { job.template('config/certs/tls.crt') }

    it 'renders the TLS certificate when provided' do
      deployment_manifest_fragment = { 'tls' => { 'cert' => 'TLS CERTIFICATE CONTENT' } }
      rendered_template = template.render(deployment_manifest_fragment)
      expect(rendered_template).to include('TLS CERTIFICATE CONTENT')
    end

    it 'renders empty when tls.cert is not provided' do
      deployment_manifest_fragment = {}
      rendered_template = template.render(deployment_manifest_fragment)
      expect(rendered_template.strip).to be_empty
    end
  end

  describe 'tls.key.erb' do
    let(:template) { job.template('config/certs/tls.key') }

    it 'renders the TLS key when provided' do
      deployment_manifest_fragment = { 'tls' => { 'key' => 'TLS KEY CONTENT' } }
      rendered_template = template.render(deployment_manifest_fragment)
      expect(rendered_template).to include('TLS KEY CONTENT')
    end

    it 'renders empty when tls.key is not provided' do
      deployment_manifest_fragment = {}
      rendered_template = template.render(deployment_manifest_fragment)
      expect(rendered_template.strip).to be_empty
    end
  end

  describe 'tls.client_ca_cert.erb' do
    let(:template) { job.template('config/certs/tls.client_ca_cert') }

    it 'renders the client CA certificate when provided' do
      deployment_manifest_fragment = { 'tls' => { 'client_ca_cert' => 'CLIENT CA CERTIFICATE CONTENT' } }
      rendered_template = template.render(deployment_manifest_fragment)
      expect(rendered_template).to include('CLIENT CA CERTIFICATE CONTENT')
    end

    it 'renders empty when tls.client_ca_cert is not provided' do
      deployment_manifest_fragment = {}
      rendered_template = template.render(deployment_manifest_fragment)
      expect(rendered_template.strip).to be_empty
    end
  end

  describe 'loggregator certificate templates' do
    describe 'loggregator_ca.crt.erb' do
      let(:template) { job.template('config/certs/loggregator/ca.crt') }

      it 'renders the loggregator CA certificate' do
        deployment_manifest_fragment = { 'loggregator' => { 'ca_cert' => 'LOGGREGATOR CA CERT' } }
        rendered_template = template.render(deployment_manifest_fragment)
        expect(rendered_template).to include('LOGGREGATOR CA CERT')
      end
    end

    describe 'loggregator_client.crt.erb' do
      let(:template) { job.template('config/certs/loggregator/client.crt') }

      it 'renders the loggregator client certificate' do
        deployment_manifest_fragment = { 'loggregator' => { 'cert' => 'LOGGREGATOR CLIENT CERT' } }
        rendered_template = template.render(deployment_manifest_fragment)
        expect(rendered_template).to include('LOGGREGATOR CLIENT CERT')
      end
    end

    describe 'loggregator_client.key.erb' do
      let(:template) { job.template('config/certs/loggregator/client.key') }

      it 'renders the loggregator client key' do
        deployment_manifest_fragment = { 'loggregator' => { 'key' => 'LOGGREGATOR CLIENT KEY' } }
        rendered_template = template.render(deployment_manifest_fragment)
        expect(rendered_template).to include('LOGGREGATOR CLIENT KEY')
      end
    end
  end
end
