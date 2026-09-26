from enum import StrEnum


class ListContractStorageStatus(StrEnum):
    ARCHIVED = "archived"
    DELETED = "deleted"
    LIVE = "live"

    def __str__(self) -> str:
        return str(self.value)
