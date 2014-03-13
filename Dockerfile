FROM cloudfoundry/inigo-ci

RUN echo "deb http://archive.ubuntu.com/ubuntu precise main universe" > /etc/apt/sources.list && \
        apt-get update && \
        apt-get upgrade

RUN locale-gen en_US.UTF-8
ENV LANG       en_US.UTF-8
ENV LC_ALL     en_US.UTF-8

ADD http://cache.ruby-lang.org/pub/ruby/2.0/ruby-2.0.0-p353.tar.gz /tmp/
RUN apt-get -y install build-essential zlib1g-dev libssl-dev libreadline6-dev libyaml-dev && \
    tar -xzf /tmp/ruby-2.0.0-p353.tar.gz && \
    (cd ruby-2.0.0-p353/ && ./configure --disable-install-doc && make && make install) && \
    rm -rf ruby-2.0.0-p353/ && \
    rm -f /tmp/ruby-2.0.0-p353.tar.gz

RUN gem install bundler
RUN gem install bosh_cli

ADD https://github.com/cloudfoundry-incubator/spiff/releases/download/v1.0/spiff_linux_amd64.zip /tmp/
RUN unzip /tmp/spiff_linux_amd64.zip -d /usr/local/bin
