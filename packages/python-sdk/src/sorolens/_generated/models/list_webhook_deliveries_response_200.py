from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.webhook_delivery import WebhookDelivery


T = TypeVar("T", bound="ListWebhookDeliveriesResponse200")


@_attrs_define
class ListWebhookDeliveriesResponse200:
    """
    Attributes:
        deliveries (list[WebhookDelivery]):
        next_cursor (str):
    """

    deliveries: list[WebhookDelivery]
    next_cursor: str
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        deliveries = []
        for deliveries_item_data in self.deliveries:
            deliveries_item = deliveries_item_data.to_dict()
            deliveries.append(deliveries_item)

        next_cursor = self.next_cursor

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "deliveries": deliveries,
                "next_cursor": next_cursor,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.webhook_delivery import WebhookDelivery

        d = dict(src_dict)
        deliveries = []
        _deliveries = d.pop("deliveries")
        for deliveries_item_data in _deliveries:
            deliveries_item = WebhookDelivery.from_dict(deliveries_item_data)

            deliveries.append(deliveries_item)

        next_cursor = d.pop("next_cursor")

        list_webhook_deliveries_response_200 = cls(
            deliveries=deliveries,
            next_cursor=next_cursor,
        )

        list_webhook_deliveries_response_200.additional_properties = d
        return list_webhook_deliveries_response_200

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
