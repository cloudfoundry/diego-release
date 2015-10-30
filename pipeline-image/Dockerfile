FROM cloudfoundry/golang-ci

RUN \
  apt-get update && \
  apt-get -y --force-yes install \
    libmysqlclient-dev \
    libpq-dev \
    libreadline6-dev \
    libsqlite3-dev \
    libssl-dev \
    libssl-dev/trusty \
    libssl1.0.0/trusty \
    libxml2-dev \
    libxslt-dev \
    libyaml-dev \
    openssl/trusty \
    zip \
    python-pip \
    zlib1g-dev && \
  apt-get clean

RUN \
  wget -q https://github.com/cloudfoundry-incubator/spiff/releases/download/v1.0.4/spiff_linux_amd64.zip -P /tmp/ && \
  unzip /tmp/spiff_linux_amd64.zip -d /usr/local/bin && \
  rm /tmp/spiff_linux_amd64.zip

RUN \
  wget -qO- https://github.com/postmodern/ruby-install/archive/v0.5.0.tar.gz | tar -C /tmp -zxf - && \
  cd /tmp/ruby-install-0.5.0 && \
  make install && \
  ruby-install --install-dir /usr/local --cleanup ruby 2.1.4 && \
  rm -rf /usr/local/src/ruby-2.1.4 && \
  rm -rf /usr/local/share/ri && \
  rm -rf /tmp/ruby-install-0.5.0

RUN \
  gem install --no-rdoc --no-ri \
    bundler \
    bosh_cli:1.2862.0 \
    debugger-ruby_core_source

RUN \
  bash -l -c "pip install awscli==1.6.6"

RUN \
  wget https://dl.bintray.com/mitchellh/vagrant/vagrant_1.6.5_x86_64.deb -P /tmp && \
  dpkg --install /tmp/vagrant_1.6.5_x86_64.deb && \
  vagrant plugin install vagrant-aws && \
  rm -rf /tmp/*

RUN \
  curl -sL http://go-cli.s3-website-us-east-1.amazonaws.com/releases/latest/cf-linux-amd64.tgz | tar zxf - -C /usr/bin

