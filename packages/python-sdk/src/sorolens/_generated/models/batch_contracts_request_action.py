from enum import StrEnum


class BatchContractsRequestAction(StrEnum):
    TAG = "tag"
    UNTRACK = "untrack"

    def __str__(self) -> str:
        return str(self.value)
