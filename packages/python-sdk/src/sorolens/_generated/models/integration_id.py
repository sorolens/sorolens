from enum import StrEnum


class IntegrationId(StrEnum):
    DISCORD = "discord"
    SLACK = "slack"
    TELEGRAM = "telegram"

    def __str__(self) -> str:
        return str(self.value)
