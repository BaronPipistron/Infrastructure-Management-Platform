from __future__ import annotations

import os
from pathlib import Path

import yaml

from app.config.models import AppConfig


APP_CONFIG_ENV_VAR = "APP_CONFIG_PATH"
PROJECT_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_APP_CONFIG_PATH = "configs/appsettings.Develop.yml"


class ConfigLoadError(RuntimeError):
    """Raised when application configuration cannot be loaded."""


def resolve_config_path(config_path: str | Path | None = None) -> Path:
    raw_path = str(config_path or os.getenv(APP_CONFIG_ENV_VAR, DEFAULT_APP_CONFIG_PATH)).strip()
    if not raw_path:
        raise ConfigLoadError("Empty app config path")

    candidate = Path(raw_path)
    if not candidate.is_absolute():
        cwd_candidate = (Path.cwd() / candidate).resolve()
        if cwd_candidate.exists():
            candidate = cwd_candidate
        else:
            candidate = (PROJECT_ROOT / candidate).resolve()
    return candidate.resolve()


def load_app_config(config_path: str | Path | None = None) -> tuple[AppConfig, Path]:
    resolved_path = resolve_config_path(config_path)
    if not resolved_path.exists():
        raise ConfigLoadError(f"Config file not found: {resolved_path}")

    try:
        with resolved_path.open("r", encoding="utf-8") as file_handle:
            data = yaml.safe_load(file_handle) or {}
    except yaml.YAMLError as exc:
        raise ConfigLoadError(f"Invalid YAML in config: {resolved_path}") from exc

    try:
        config = AppConfig.model_validate(data)
    except Exception as exc:  # pragma: no cover - Pydantic provides detailed validation
        raise ConfigLoadError(f"Config validation failed for: {resolved_path}") from exc

    return config, resolved_path
