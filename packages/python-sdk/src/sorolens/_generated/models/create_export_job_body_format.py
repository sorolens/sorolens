from enum import StrEnum


class CreateExportJobBodyFormat(StrEnum):
    CSV = "csv"
    NDJSON = "ndjson"

    def __str__(self) -> str:
        return str(self.value)
