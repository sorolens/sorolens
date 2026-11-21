from enum import StrEnum


class ListMaintenanceWindowsScope(StrEnum):
    CONTRACT = "contract"
    GLOBAL = "global"
    GROUP = "group"

    def __str__(self) -> str:
        return str(self.value)
