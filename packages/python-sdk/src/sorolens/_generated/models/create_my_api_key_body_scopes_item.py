from enum import StrEnum


class CreateMyApiKeyBodyScopesItem(StrEnum):
    READCONTRACTS = "read:contracts"
    READWATCHDOG = "read:watchdog"
    WRITECONTRACTS = "write:contracts"

    def __str__(self) -> str:
        return str(self.value)
