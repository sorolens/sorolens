from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.alert_subscription_channel_type import AlertSubscriptionChannelType
from ..models.alert_subscription_severity_filter import AlertSubscriptionSeverityFilter

T = TypeVar("T", bound="AlertSubscription")


@_attrs_define
class AlertSubscription:
    """A notification channel subscribed to a contract's alerts. Secrets are
    never returned: Slack and Discord webhook URLs are masked
    (e.g. `https://hooks.slack.com/services/***`) and the PagerDuty
    routing key is reported only via `has_routing_key`.

        Attributes:
            id (str):
            contract_id (str):
            channel_type (AlertSubscriptionChannelType):
            webhook_url (str):
            has_routing_key (bool):
            severity_filter (AlertSubscriptionSeverityFilter):
            created_at (datetime.datetime):
            updated_at (datetime.datetime):
    """

    id: str
    contract_id: str
    channel_type: AlertSubscriptionChannelType
    webhook_url: str
    has_routing_key: bool
    severity_filter: AlertSubscriptionSeverityFilter
    created_at: datetime.datetime
    updated_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = self.id

        contract_id = self.contract_id

        channel_type = self.channel_type.value

        webhook_url = self.webhook_url

        has_routing_key = self.has_routing_key

        severity_filter = self.severity_filter.value

        created_at = self.created_at.isoformat()

        updated_at = self.updated_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "contract_id": contract_id,
                "channel_type": channel_type,
                "webhook_url": webhook_url,
                "has_routing_key": has_routing_key,
                "severity_filter": severity_filter,
                "created_at": created_at,
                "updated_at": updated_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = d.pop("id")

        contract_id = d.pop("contract_id")

        channel_type = AlertSubscriptionChannelType(d.pop("channel_type"))

        webhook_url = d.pop("webhook_url")

        has_routing_key = d.pop("has_routing_key")

        severity_filter = AlertSubscriptionSeverityFilter(d.pop("severity_filter"))

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        updated_at = datetime.datetime.fromisoformat(d.pop("updated_at"))

        alert_subscription = cls(
            id=id,
            contract_id=contract_id,
            channel_type=channel_type,
            webhook_url=webhook_url,
            has_routing_key=has_routing_key,
            severity_filter=severity_filter,
            created_at=created_at,
            updated_at=updated_at,
        )

        alert_subscription.additional_properties = d
        return alert_subscription

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
