FROM cloudfoundry/inigo-ci

RUN echo "deb http://archive.ubuntu.com/ubuntu precise main universe" > /etc/apt/sources.list && \
        apt-get update && \
        apt-get upgrade

RUN locale-gen en_US.UTF-8
ENV LANG       en_US.UTF-8
ENV LC_ALL     en_US.UTF-8

ADD https://github.com/cloudfoundry-incubator/spiff/releases/download/v1.0/spiff_linux_amd64.zip /tmp/
RUN ls /tmp
RUN unzip /tmp/spiff_linux_amd64.zip -d /usr/local/bin
RUN rm /tmp/spiff_linux_amd64.zip

# bosh_cli (nokogiri)
RUN apt-get -y install libxml2-dev libxslt-dev libcurl4-openssl-dev

# ccng prepackaging
RUN apt-get -y install libmysqlclient-dev libpq-dev libsqlite3-dev

ADD http://cache.ruby-lang.org/pub/ruby/1.9/ruby-1.9.3-p545.tar.gz /tmp/
RUN apt-get -y install build-essential zlib1g-dev libssl-dev libreadline6-dev libyaml-dev && \
    tar -xzf /tmp/ruby-1.9.3-p545.tar.gz && \
    (cd ruby-1.9.3-p545/ && ./configure --disable-install-doc && make && make install) && \
    rm -rf ruby-1.9.3-p545/ && \
    rm -f /tmp/ruby-1.9.3-p545.tar.gz

RUN gem install bundler --no-rdoc --no-ri

RUN gem install bosh_cli --no-rdoc --no-ri

# warden prepackaging
RUN gem install rake -v 0.9.2.2 --no-rdoc --no-ri
