from enum import StrEnum


class CreateMaintenanceWindowRequestRecurrence(StrEnum):
    DAILY = "daily"
    NONE = "none"
    WEEKLY = "weekly"

    def __str__(self) -> str:
        return str(self.value)
