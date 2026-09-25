from enum import StrEnum

class CreateApiKeyBodyScopesItem(StrEnum):
    ADMIN = "admin:*"
    READCONTRACTS = "read:contracts"
    READWATCHDOG = "read:watchdog"
    WRITECONTRACTS = "write:contracts"

    def __str__(self) -> str:
        return str(self.value)
