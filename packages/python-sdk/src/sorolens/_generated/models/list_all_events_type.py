from enum import StrEnum


class ListAllEventsType(StrEnum):
    CONTRACT = "contract"
    DIAGNOSTIC = "diagnostic"
    SYSTEM = "system"

    def __str__(self) -> str:
        return str(self.value)
