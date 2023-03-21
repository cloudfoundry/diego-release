# frozen_string_literal: true
# rubocop: disable Layout/LineLength
# rubocop: disable Metrics/BlockLength
require 'rspec'
require 'json'
require 'bosh/template/test'

describe 'auctioneer' do 
    let(:release_path) { File.join(File.dirname(__FILE__), '..') }
    let(:release) { Bosh::Template::Test::ReleaseDir.new(release_path)}
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
    end 
end