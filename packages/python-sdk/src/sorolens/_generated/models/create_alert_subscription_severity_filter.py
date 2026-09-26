from enum import StrEnum


class CreateAlertSubscriptionSeverityFilter(StrEnum):
    CRITICAL = "Critical"
    INFO = "Info"
    WARNING = "Warning"

    def __str__(self) -> str:
        return str(self.value)
