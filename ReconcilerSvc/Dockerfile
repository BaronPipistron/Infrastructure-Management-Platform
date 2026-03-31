FROM python:3.12-slim

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends openssh-client sshpass \
    && rm -rf /var/lib/apt/lists/*

COPY pyproject.toml README.md ./
COPY app ./app
COPY ansible ./ansible
COPY configs ./configs

RUN pip install --no-cache-dir .

RUN useradd --create-home --uid 10001 appuser \
    && mkdir -p /var/lib/reconciler-svc \
    && chown -R appuser:appuser /app /var/lib/reconciler-svc

ENV APP_CONFIG_PATH=/app/configs/appsettings.Prod.yml

VOLUME ["/var/lib/reconciler-svc"]
EXPOSE 8082

USER appuser

ENTRYPOINT ["reconciler-svc"]
