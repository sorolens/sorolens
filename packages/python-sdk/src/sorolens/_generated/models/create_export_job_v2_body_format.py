from enum import StrEnum


class CreateExportJobV2BodyFormat(StrEnum):
    CSV = "csv"
    NDJSON = "ndjson"

    def __str__(self) -> str:
        return str(self.value)
