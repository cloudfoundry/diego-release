# frozen_string_literal: true

# rubocop: disable Metrics/BlockLength
require 'rspec'
require 'json'
require 'bosh/template/test'

describe 'rep' do
  let(:release_path) { File.join(File.dirname(__FILE__), '..') }
  let(:release) { Bosh::Template::Test::ReleaseDir.new(release_path) }
  let(:job) { release.job('rep') }

  let(:deployment_manifest_fragment) do
    {
      'bpm' => {
        'enabled' => 'true'
      },
      'diego' => {
        'executor' => {
          'instance_identity_ca_cert' => 'CA CERT',
          'instance_identity_key' => 'CA KEY'
        },
        'rep' => {
          'max_containers' => 250,
          'locket' => {
            'client_keepalive_time' => 10,
            'client_keepalive_timeout' => 22
          },
          'preloaded_rootfses' => %w[
            cflinuxfs3
            cflinuxfs4
          ]
        }
      },
      'containers' => {
        'proxy' => {
          'enabled' => 'true',
          'require_and_verify_client_certificates' => 'true',
          'trusted_ca_certificates' => [
            'GOROUTER CA',
            'SSH PROXY CA'
          ],
          'verify_subject_alt_name' => [
            'gorouter.service.cf.internal',
            'ssh-proxy.service.cf.internal'
          ]
        },
        'trusted_ca_certificate' => [
          'DIEGO INSTANCE CA',
          'CREDHUB CA',
          'UAA CA'
        ]
      },
      'enable_consul_service_registration' => 'false',
      'enable_declarative_healthcheck' => 'true',
      'loggregator' => {},
      'tls' => {
        'ca_cert' => 'CA CERT',
        'cert' => 'CERT',
        'key' => 'KEY'
      },
      'logging' => {
        'format' => {
          'timestamp' => 'rfc3339'
        }
      }
    }
  end
  
  let(:rendered_template) { template.render(deployment_manifest_fragment) }

  describe 'rep.json.erb' do
    let(:template) { job.template('config/rep.json') }
    
    context 'check if locket keepalive time is bigger than the timeout' do
      it 'fails if the keepalive time is bigger than timeout' do
        deployment_manifest_fragment['diego']['rep']['locket']['client_keepalive_time'] = 23
        expect do
          rendered_template
        end.to raise_error(/The locket client keepalive time property should not be larger than the timeout/)
      end
    end

    it 'excludes the newer cpu_entitlement metric by default for backwards compatibility' do
      deployment_manifest_fragment['loggregator']['use_v2_api'] = true
      expect(JSON.parse(rendered_template)['loggregator']['loggregator_app_metric_exclusion_filter']).to eq(%w[cpu_entitlement])
    end

    context 'when specific app metrics are configured to be excluded' do
      it 'configures the rep to exclude them' do
        deployment_manifest_fragment['loggregator']['use_v2_api'] = true
        deployment_manifest_fragment['loggregator']['app_metric_exclusion_filter']= %w[absolute_entitlement absolute_usage]
        expect(JSON.parse(rendered_template)['loggregator']['loggregator_app_metric_exclusion_filter']).to eq(%w[absolute_entitlement absolute_usage])
      end
    end
  end

  describe 'setup_mounted_data_dirs.erb' do
    let(:template) { job.template('bin/setup_mounted_data_dirs') }
   
    context 'checks the max_containers value' do 
      it 'raises an error if max_containers is <= 0' do
        deployment_manifest_fragment['diego']['rep']['max_containers'] = -10
        expect do 
          rendered_template
        end.to raise_error(/The max_containers prop should be a positive integer/)
      end
    end
  end  
end
