# syntax=docker/dockerfile:1

ARG GO_VERSION=1.26
ARG DISTRO_NAME=bookworm

FROM golang:${GO_VERSION}-${DISTRO_NAME}

ARG DISTRO_NAME

# System dependencies needed to develop, build, and run AnyCable-Go and the
# Ruby-based conformance test suite. mruby toolchain is intentionally omitted —
# the dev image uses MRUBY_GO=false to skip the mrb build tag.
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    rm -f /etc/apt/apt.conf.d/docker-clean; \
    echo 'Binary::apt::APT::Keep-Downloaded-Packages "true";' > /etc/apt/apt.conf.d/keep-cache; \
    apt-get update -qq && \
    DEBIAN_FRONTEND=noninteractive apt-get install -yq --no-install-recommends \
      build-essential \
      ca-certificates \
      curl \
      git \
      less \
      lsof \
      ruby-full \
      ruby-dev \
      libffi-dev \
      libssl-dev \
      libyaml-dev \
      protobuf-compiler \
      redis-tools \
      bubblewrap \
      vim \
      bash-completion

# Install Claude CLI (used by `dip claude`).
RUN curl -fsSL https://claude.ai/install.sh | bash
ENV PATH=/root/.local/bin:${PATH}
ENV IS_SANDBOX=1

# Ruby/Bundler configuration. Gems live in a named volume.
ENV LANG=C.UTF-8 \
    BUNDLE_JOBS=4 \
    BUNDLE_RETRY=3 \
    BUNDLE_PATH=/usr/local/bundle \
    GEM_HOME=/usr/local/bundle \
    GEM_PATH=/usr/local/bundle \
    BUNDLE_APP_CONFIG=/usr/local/bundle/.bundle

RUN gem install --no-document bundler

ENV PATH=/usr/local/bundle/bin:${PATH}

# In the Docker dev env we always skip the mrb tag — no mruby toolchain inside.
ENV MRUBY_GO=false \
    ANYCABLE_REDIS_URL=redis://redis:6379/0

WORKDIR /app

EXPOSE 8080

CMD ["/bin/bash"]
