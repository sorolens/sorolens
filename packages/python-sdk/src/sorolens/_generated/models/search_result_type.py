from enum import StrEnum


class SearchResultType(StrEnum):
    CONTRACT = "contract"
    EVENT = "event"
    FUNCTION = "function"

    def __str__(self) -> str:
        return str(self.value)
