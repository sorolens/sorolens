from enum import StrEnum


class CreateExportJobV2BodyType(StrEnum):
    EVENTS = "events"
    SNAPSHOT = "snapshot"

    def __str__(self) -> str:
        return str(self.value)
