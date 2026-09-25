from enum import StrEnum

class ContractStatus(StrEnum):
    ACTIVE = "active"
    BACKFILLING = "backfilling"
    ERROR = "error"
    PAUSED = "paused"
    PENDING = "pending"

    def __str__(self) -> str:
        return str(self.value)
