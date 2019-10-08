FROM openjdk:11.0.4-jre-slim

ENV CONF_STAGING_DIRECTORY=/staging/conf
ENV LIB_STAGING_DIRECTORY=/staging/lib

RUN mkdir -p $CONF_STAGING_DIRECTORY $LIB_STAGING_DIRECTORY

# install extra libraries
COPY build/extraLib/* $LIB_STAGING_DIRECTORY/

# install default prometheus and jolokia config
COPY conf/* $CONF_STAGING_DIRECTORY/

# install bootstrapper application
COPY bootstrapper/build/libs/cassandra-bootstrapper.jar /cassandra-bootstrapper.jar

# install bootstrap entry point
COPY bootstrapper/bootstrap.sh /
ENTRYPOINT ["/bootstrap.sh"]
