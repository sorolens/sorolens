from enum import StrEnum


class CreatePushSubscriptionBodyFiltersSeverity(StrEnum):
    CRITICAL = "Critical"
    INFO = "Info"
    WARNING = "Warning"

    def __str__(self) -> str:
        return str(self.value)
