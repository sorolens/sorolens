from enum import StrEnum


class ListContractStorageDurability(StrEnum):
    INSTANCE = "instance"
    PERSISTENT = "persistent"
    TEMPORARY = "temporary"

    def __str__(self) -> str:
        return str(self.value)
