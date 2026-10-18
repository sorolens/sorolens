from enum import StrEnum


class ListContractsSort(StrEnum):
    ADDED_AT = "added_at"
    ID = "id"
    LABEL = "label"
    NETWORK = "network"
    STATUS = "status"

    def __str__(self) -> str:
        return str(self.value)
