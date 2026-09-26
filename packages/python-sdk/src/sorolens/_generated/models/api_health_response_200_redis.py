from enum import StrEnum


class ApiHealthResponse200Redis(StrEnum):
    CONNECTED = "connected"
    UNREACHABLE = "unreachable"

    def __str__(self) -> str:
        return str(self.value)
