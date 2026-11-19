from enum import StrEnum


class V2ListInvocationsStatus(StrEnum):
    FAILED = "FAILED"
    NOT_FOUND = "NOT_FOUND"
    SUCCESS = "SUCCESS"

    def __str__(self) -> str:
        return str(self.value)
