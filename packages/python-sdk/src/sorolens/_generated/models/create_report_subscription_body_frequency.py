from enum import StrEnum


class CreateReportSubscriptionBodyFrequency(StrEnum):
    DAILY = "daily"
    WEEKLY = "weekly"

    def __str__(self) -> str:
        return str(self.value)
