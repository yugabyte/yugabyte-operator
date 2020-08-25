FROM registry.access.redhat.com/ubi7/ubi-minimal:latest
MAINTAINER Yugabyte dev-admin-cloud@yugabyte.com

ENV OPERATOR=/usr/local/bin/yugabyte-operator \
    USER_UID=1001 \
    USER_NAME=yugabyte-operator

LABEL name="YugabyteDB" \
      maintainer="dev-admin-cloud@yugabyte.com" \
      vendor="Yugabyte Inc" \
      release="1" \
      version="0.9.0" \
      summary="YugabyteDB Operator Image" \
      description="YugabyteDB Operator Image"

COPY licenses /licenses

# install operator binary
COPY build/_output/bin/yugabyte-operator ${OPERATOR}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
