from enum import StrEnum


class SlackMessageResponseType(StrEnum):
    EPHEMERAL = "ephemeral"

    def __str__(self) -> str:
        return str(self.value)
