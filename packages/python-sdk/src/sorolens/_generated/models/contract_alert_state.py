from enum import StrEnum


class ContractAlertState(StrEnum):
    ACKNOWLEDGED = "acknowledged"
    OPEN = "open"
    RESOLVED = "resolved"
    SNOOZED = "snoozed"

    def __str__(self) -> str:
        return str(self.value)
