from enum import StrEnum


class ListContractsSort(StrEnum):
    ADDED_AT = "added_at"
    EVENTS_COUNT = "events_count"
    LAST_ACTIVITY = "last_activity"

    def __str__(self) -> str:
        return str(self.value)
