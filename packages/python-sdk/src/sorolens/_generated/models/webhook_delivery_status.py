from enum import StrEnum


class WebhookDeliveryStatus(StrEnum):
    DELIVERED = "delivered"
    FAILED = "failed"
    PENDING = "pending"

    def __str__(self) -> str:
        return str(self.value)
