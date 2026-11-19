from enum import StrEnum


class V2ListEventsType(StrEnum):
    CONTRACT = "contract"
    SYSTEM = "system"

    def __str__(self) -> str:
        return str(self.value)
