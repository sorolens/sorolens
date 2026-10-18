from enum import StrEnum


class V2StorageEntryDurability(StrEnum):
    INSTANCE = "instance"
    PERSISTENT = "persistent"
    TEMPORARY = "temporary"

    def __str__(self) -> str:
        return str(self.value)
