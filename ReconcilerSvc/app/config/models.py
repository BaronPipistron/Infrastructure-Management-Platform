from __future__ import annotations

from pathlib import Path
from typing import Any

from pydantic import BaseModel, Field, model_validator


class ServerConfig(BaseModel):
    host: str = "0.0.0.0"
    port: int = Field(default=8082, ge=1, le=65535)


class LoggingConfig(BaseModel):
    level: str = "INFO"


class ReconcilerRuntimeConfig(BaseModel):
    max_parallel_reconciliations: int = Field(default=2, ge=1)
    queue_capacity: int = Field(default=100, ge=1)
    enqueue_timeout_seconds: float = Field(default=1.0, gt=0)


class AnsibleSSHConfig(BaseModel):
    user_env_var: str = "ANSIBLE_SSH_USER"
    private_key_path_env_var: str = "ANSIBLE_SSH_PRIVATE_KEY_PATH"
    port_env_var: str = "ANSIBLE_SSH_PORT"
    host_key_checking_env_var: str = "ANSIBLE_HOST_KEY_CHECKING"
    default_port: int = Field(default=22, ge=1, le=65535)


class AnsibleConfig(BaseModel):
    playbooks_base_dir: str = "./ansible"
    private_data_dir: str = "./.ansible-runner"
    run_timeout_seconds: int = Field(default=900, ge=1)
    ssh: AnsibleSSHConfig = Field(default_factory=AnsibleSSHConfig)


class OperatorRuntimeConfig(BaseModel):
    enabled: bool = True
    playbook: str
    default_parameters: dict[str, Any] = Field(default_factory=dict)


class OperatorsConfig(BaseModel):
    node_exporter: OperatorRuntimeConfig = Field(
        default_factory=lambda: OperatorRuntimeConfig(playbook="node_exporter/playbook.yml")
    )
    cadvisor: OperatorRuntimeConfig = Field(
        default_factory=lambda: OperatorRuntimeConfig(playbook="cadvisor/playbook.yml")
    )


class AppConfig(BaseModel):
    server: ServerConfig = Field(default_factory=ServerConfig)
    logging: LoggingConfig = Field(default_factory=LoggingConfig)
    reconciler: ReconcilerRuntimeConfig = Field(default_factory=ReconcilerRuntimeConfig)
    ansible: AnsibleConfig = Field(default_factory=AnsibleConfig)
    operators: OperatorsConfig = Field(default_factory=OperatorsConfig)

    @model_validator(mode="after")
    def validate_relative_playbook_paths(self) -> "AppConfig":
        for item in (self.operators.node_exporter, self.operators.cadvisor):
            if Path(item.playbook).is_absolute():
                raise ValueError("operator.playbook must be relative to ansible.playbooks_base_dir")
        return self
