# frozen_string_literal: true

# rubocop: disable Metrics/BlockLength
require 'rspec'
require 'json'
require 'bosh/template/test'

describe 'rep' do
  let(:release_path) { File.join(File.dirname(__FILE__), '..') }
  let(:release) { Bosh::Template::Test::ReleaseDir.new(release_path) }
  let(:job) { release.job('rep_windows') }

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
            cflinuxfs3:/var/vcap/packages/cflinuxfs3/rootfs.tar
            cflinuxfs4:/var/vcap/packages/cflinuxfs4/rootfs.tar
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

    context 'lock_ttl' do
      it 'defaults to 15s' do
        expect(JSON.parse(rendered_template)['lock_ttl']).to eq('15s')
      end

      it 'is configurable' do
        deployment_manifest_fragment['diego']['rep']['locket']['lock_ttl'] = 30
        expect(JSON.parse(rendered_template)['lock_ttl']).to eq('30s')
      end
    end

    context 'check if locket keepalive time is bigger than the timeout' do
      it 'fails if the keepalive time is bigger than timeout' do
        deployment_manifest_fragment['diego']['rep']['locket']['client_keepalive_time'] = 23
        expect do
          rendered_template
        end.to raise_error(/The locket client keepalive time property should not be larger than the timeout/)
      end
    end

    it 'excludes the newer cpu_entitlement metric by default for backwards compatibility' do
      expect(JSON.parse(rendered_template)['loggregator']['loggregator_app_metric_exclusion_filter']).to eq(%w[cpu_entitlement])
    end

    context 'when specific app metrics are configured to be excluded' do
      it 'configures the rep to exclude them' do
        deployment_manifest_fragment['loggregator']['app_metric_exclusion_filter']= %w[absolute_entitlement absolute_usage]
        expect(JSON.parse(rendered_template)['loggregator']['loggregator_app_metric_exclusion_filter']).to eq(%w[absolute_entitlement absolute_usage])
      end
    end

    context 'extra_root_fs_dir' do
      it 'is set to /var/vcap/data/rootfses by default' do
        expect(JSON.parse(rendered_template)['extra_root_fs_dir']).to eq('/var/vcap/data/rootfses')
      end

      it 'is configurable' do
        deployment_manifest_fragment['diego']['rep']['extra_root_fs_dir'] = '/var/meow/vcap/meow'
        expect(JSON.parse(rendered_template)['extra_root_fs_dir']).to eq('/var/meow/vcap/meow')
      end
    end

    context 'sidecar_root_fs_path' do
      it 'is empty by default' do
        expect(JSON.parse(rendered_template)['sidecar_root_fs_path']).to eq('')
      end

      it 'is configurable' do
        deployment_manifest_fragment['diego']['rep']['sidecar_rootfs_path'] = '/var/meow/vcap/meow'
        expect(JSON.parse(rendered_template)['sidecar_root_fs_path']).to eq('/var/meow/vcap/meow')
      end
    end

    context 'sidecar_root_fs' do
      it 'is empty by default' do
        expect(JSON.parse(rendered_template)['sidecar_root_fs']).to eq('')
      end

      it 'is configurable' do
        deployment_manifest_fragment['diego']['rep']['sidecar_rootfs'] = 'cflinuxfs4'
        expect(JSON.parse(rendered_template)['sidecar_root_fs']).to eq('cflinuxfs4')
      end
    end

    context 'cell_annotations' do
      it 'includes the deployment_guid based on the bosh deployment name' do
        expect(JSON.parse(rendered_template)['cell_annotations']).to eq('deployment_guid' => 'my-deployment')
      end

      it 'merges the deployment_guid into operator-provided annotations' do
        deployment_manifest_fragment['diego']['rep']['cell_annotations'] = { 'team' => 'diego' }
        expect(JSON.parse(rendered_template)['cell_annotations']).to eq(
          'team' => 'diego',
          'deployment_guid' => 'my-deployment'
        )
      end

      it 'always overrides an operator-provided deployment_guid with the bosh deployment name' do
        deployment_manifest_fragment['diego']['rep']['cell_annotations'] = { 'deployment_guid' => 'user-provided' }
        expect(JSON.parse(rendered_template)['cell_annotations']).to eq('deployment_guid' => 'my-deployment')
      end

      it 'reflects a custom bosh deployment name' do
        spec = Bosh::Template::Test::InstanceSpec.new(deployment: 'cf-4f4d28d91b17601281d4')
        rendered = template.render(deployment_manifest_fragment, spec: spec)
        expect(JSON.parse(rendered)['cell_annotations']).to eq('deployment_guid' => 'cf-4f4d28d91b17601281d4')
      end
    end

    context 'min_cache_partition_free_bytes' do
      it 'defaults to 5368709120 (5GB)' do
        expect(JSON.parse(rendered_template)['min_cache_partition_free_bytes']).to eq(5_368_709_120)
      end

      it 'is configurable' do
        deployment_manifest_fragment['diego']['executor']['min_cache_partition_free_bytes'] = 1_073_741_824
        expect(JSON.parse(rendered_template)['min_cache_partition_free_bytes']).to eq(1_073_741_824)
      end
    end
  end
end
