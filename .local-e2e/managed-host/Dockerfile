FROM debian:12-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
      ca-certificates \
      curl \
      docker.io \
      iproute2 \
      openssh-server \
      procps \
      python3 \
      python3-apt \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /var/run/sshd /root/.ssh \
    && chmod 700 /root/.ssh \
    && sed -ri 's/^#?PermitRootLogin .*/PermitRootLogin prohibit-password/' /etc/ssh/sshd_config \
    && sed -ri 's/^#?PubkeyAuthentication .*/PubkeyAuthentication yes/' /etc/ssh/sshd_config \
    && sed -ri 's/^#?PasswordAuthentication .*/PasswordAuthentication no/' /etc/ssh/sshd_config \
    && sed -ri 's/^#?UsePAM .*/UsePAM no/' /etc/ssh/sshd_config

COPY entrypoint.sh /usr/local/bin/local-e2e-entrypoint.sh
RUN chmod 0755 /usr/local/bin/local-e2e-entrypoint.sh

EXPOSE 22

ENTRYPOINT ["/usr/local/bin/local-e2e-entrypoint.sh"]
