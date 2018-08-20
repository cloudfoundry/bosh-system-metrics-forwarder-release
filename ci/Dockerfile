FROM ubuntu:xenial AS fetch

RUN apt-get update && apt-get install -y curl python

RUN curl https://s3.amazonaws.com/bosh-cli-artifacts/bosh-cli-5.1.1-linux-amd64 > /tmp/bosh-cli-5.1.1-linux-amd64
RUN chmod +x /tmp/bosh-cli-5.1.1-linux-amd64

# Downloading gcloud package
RUN curl https://dl.google.com/dl/cloudsdk/release/google-cloud-sdk.tar.gz > /tmp/google-cloud-sdk.tar.gz

# Installing the package
RUN mkdir -p /usr/local/gcloud \
  && tar -C /usr/local/gcloud -xvf /tmp/google-cloud-sdk.tar.gz \
  && /usr/local/gcloud/google-cloud-sdk/install.sh

FROM ubuntu:xenial

RUN apt-get update && apt-get install -y python git make build-essential ruby

COPY --from=fetch /tmp/bosh-cli-5.1.1-linux-amd64 /bin/bosh
COPY --from=fetch /usr/local/gcloud/google-cloud-sdk/ /tmp/google-cloud-sdk

ENV PATH="/tmp/google-cloud-sdk/bin:${PATH}"

