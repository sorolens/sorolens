from enum import StrEnum


class CreateAlertSubscriptionChannelType(StrEnum):
    DISCORD = "discord"
    PAGERDUTY = "pagerduty"
    SLACK = "slack"
    WEBHOOK = "webhook"

    def __str__(self) -> str:
        return str(self.value)
