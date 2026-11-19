from enum import StrEnum


class ApiHealthResponse200Db(StrEnum):
    CONNECTED = "connected"
    UNREACHABLE = "unreachable"

    def __str__(self) -> str:
        return str(self.value)
