from enum import StrEnum


class CompareContractsWindow(StrEnum):
    VALUE_0 = "24h"
    VALUE_1 = "7d"
    VALUE_2 = "30d"

    def __str__(self) -> str:
        return str(self.value)
