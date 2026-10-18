from enum import StrEnum


class BatchContractsResultAction(StrEnum):
    TAG = "tag"
    UNTRACK = "untrack"

    def __str__(self) -> str:
        return str(self.value)
