from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.create_alert_subscription_channel_type import (
    CreateAlertSubscriptionChannelType,
)
from ..models.create_alert_subscription_severity_filter import (
    CreateAlertSubscriptionSeverityFilter,
)
from ..types import UNSET, Unset

T = TypeVar("T", bound="CreateAlertSubscription")


@_attrs_define
class CreateAlertSubscription:
    """
    Attributes:
        contract_id (str):
        channel_type (CreateAlertSubscriptionChannelType | Unset):  Default: CreateAlertSubscriptionChannelType.WEBHOOK.
        webhook_url (str | Unset): Required for webhook (http or https), slack and discord (https
            incoming webhook URL). Optional for pagerduty, where it defaults
            to https://events.pagerduty.com/v2/enqueue.
        routing_key (str | Unset): PagerDuty integration key. Required for pagerduty, rejected otherwise.
        severity_filter (CreateAlertSubscriptionSeverityFilter | Unset):  Default:
            CreateAlertSubscriptionSeverityFilter.CRITICAL.
    """

    contract_id: str
    channel_type: CreateAlertSubscriptionChannelType | Unset = (
        CreateAlertSubscriptionChannelType.WEBHOOK
    )
    webhook_url: str | Unset = UNSET
    routing_key: str | Unset = UNSET
    severity_filter: CreateAlertSubscriptionSeverityFilter | Unset = (
        CreateAlertSubscriptionSeverityFilter.CRITICAL
    )
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        channel_type: str | Unset = UNSET
        if not isinstance(self.channel_type, Unset):
            channel_type = self.channel_type.value

        webhook_url = self.webhook_url

        routing_key = self.routing_key

        severity_filter: str | Unset = UNSET
        if not isinstance(self.severity_filter, Unset):
            severity_filter = self.severity_filter.value

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "contract_id": contract_id,
            }
        )
        if channel_type is not UNSET:
            field_dict["channel_type"] = channel_type
        if webhook_url is not UNSET:
            field_dict["webhook_url"] = webhook_url
        if routing_key is not UNSET:
            field_dict["routing_key"] = routing_key
        if severity_filter is not UNSET:
            field_dict["severity_filter"] = severity_filter

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        _channel_type = d.pop("channel_type", UNSET)
        channel_type: CreateAlertSubscriptionChannelType | Unset
        if isinstance(_channel_type, Unset):
            channel_type = UNSET
        else:
            channel_type = CreateAlertSubscriptionChannelType(_channel_type)

        webhook_url = d.pop("webhook_url", UNSET)

        routing_key = d.pop("routing_key", UNSET)

        _severity_filter = d.pop("severity_filter", UNSET)
        severity_filter: CreateAlertSubscriptionSeverityFilter | Unset
        if isinstance(_severity_filter, Unset):
            severity_filter = UNSET
        else:
            severity_filter = CreateAlertSubscriptionSeverityFilter(_severity_filter)

        create_alert_subscription = cls(
            contract_id=contract_id,
            channel_type=channel_type,
            webhook_url=webhook_url,
            routing_key=routing_key,
            severity_filter=severity_filter,
        )

        create_alert_subscription.additional_properties = d
        return create_alert_subscription

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
