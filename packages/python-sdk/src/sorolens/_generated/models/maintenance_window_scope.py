from enum import StrEnum


class MaintenanceWindowScope(StrEnum):
    CONTRACT = "contract"
    GLOBAL = "global"
    GROUP = "group"

    def __str__(self) -> str:
        return str(self.value)
