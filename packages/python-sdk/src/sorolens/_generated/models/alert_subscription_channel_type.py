from enum import StrEnum


class AlertSubscriptionChannelType(StrEnum):
    DISCORD = "discord"
    PAGERDUTY = "pagerduty"
    SLACK = "slack"
    WEBHOOK = "webhook"

    def __str__(self) -> str:
        return str(self.value)
