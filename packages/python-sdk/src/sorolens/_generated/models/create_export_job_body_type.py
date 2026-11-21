from enum import StrEnum


class CreateExportJobBodyType(StrEnum):
    EVENTS = "events"
    SNAPSHOT = "snapshot"

    def __str__(self) -> str:
        return str(self.value)
