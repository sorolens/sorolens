from enum import StrEnum


class TTLOperationOpType(StrEnum):
    EXTEND_TTL = "extend_ttl"
    RESTORE_FOOTPRINT = "restore_footprint"

    def __str__(self) -> str:
        return str(self.value)
