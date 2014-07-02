FROM cloudfoundry/inigo-ci

RUN echo "deb http://archive.ubuntu.com/ubuntu trusty main universe" > /etc/apt/sources.list && \
        apt-get update && \
        apt-get upgrade

RUN locale-gen en_US.UTF-8
ENV LANG       en_US.UTF-8
ENV LC_ALL     en_US.UTF-8

ADD https://github.com/cloudfoundry-incubator/spiff/releases/download/v1.0/spiff_linux_amd64.zip /tmp/
RUN unzip /tmp/spiff_linux_amd64.zip -d /usr/local/bin
RUN rm /tmp/spiff_linux_amd64.zip

# bosh_cli (nokogiri)
RUN apt-get update && apt-get -y install libxml2-dev libxslt-dev

# ccng prepackaging
RUN apt-get update && apt-get -y --force-yes install libssl1.0.0/trusty libssl-dev/trusty openssl/trusty libmysqlclient-dev libpq-dev libsqlite3-dev

RUN apt-get update && apt-get -y install build-essential zlib1g-dev libssl-dev libreadline6-dev libyaml-dev 

ADD https://github.com/postmodern/ruby-install/archive/v0.4.1.tar.gz /tmp/
RUN tar zxf /tmp/v0.4.1.tar.gz -C /tmp
RUN cd /tmp/ruby-install-0.4.1 && make install
RUN rm -rf /tmp/ruby-install-0.4.1

RUN ruby-install -i /usr/local/ ruby 1.9.3-p545

RUN gem install bundler --no-rdoc --no-ri

RUN gem install bosh_cli --no-rdoc --no-ri

# warden prepackaging
RUN gem install rake -v 0.9.2.2 --no-rdoc --no-ri

# this gem provides the headers to other gems.
#
# no, really.
RUN gem install debugger-ruby_core_source --no-rdoc --no-ri

# curl for cats
RUN apt-get update && apt-get -y install curl

# cf (gcf) for cats
RUN curl -L http://go-cli.s3-website-us-east-1.amazonaws.com/releases/latest/cf-linux-amd64.tgz | tar zxf - -C /usr/bin
RUN cp /usr/bin/cf /usr/bin/gcf
