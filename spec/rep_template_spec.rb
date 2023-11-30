# frozen_string_literal: true

# rubocop: disable Metrics/BlockLength
require 'rspec'
require 'json'
require 'bosh/template/test'

describe 'rep' do
  let(:release_path) { File.join(File.dirname(__FILE__), '..') }
  let(:release) { Bosh::Template::Test::ReleaseDir.new(release_path) }
  let(:job) { release.job('rep') }

  describe 'rep.json.erb' do
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
          'min_instance_memory_mb' => 128,
          'max_instance_memory_mb' => 8192,
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
        'loggregator' => 'LOGREGATOR PROPS',
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

    let(:template) { job.template('config/rep.json') }
    let(:rendered_template) { template.render(deployment_manifest_fragment) }

    context 'check if locket keepalive time is bigger than the timeout' do
      it 'fails if the keepalive time is bigger than timeout' do
        deployment_manifest_fragment['diego']['rep']['locket']['client_keepalive_time'] = 23
        expect do
          rendered_template
        end.to raise_error(/The locket client keepalive time property should not be larger than the timeout/)
      end
    end

    context 'check the value of min_instance_memory_mb' do
      it 'fails if min_instance_memory_mb is less than 0' do
        deployment_manifest_fragment['containers']['min_instance_memory_mb'] = -100
        expect do
          rendered_template
        end.to raise_error(/Min_instance_memory_mb has to be larger than 0/)
      end
    end
  end
end
