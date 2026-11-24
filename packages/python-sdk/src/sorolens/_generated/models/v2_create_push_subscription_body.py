from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.v2_create_push_subscription_body_filters import (
        V2CreatePushSubscriptionBodyFilters,
    )
    from ..models.v2_create_push_subscription_body_keys import (
        V2CreatePushSubscriptionBodyKeys,
    )


T = TypeVar("T", bound="V2CreatePushSubscriptionBody")


@_attrs_define
class V2CreatePushSubscriptionBody:
    """
    Attributes:
        endpoint (str):
        keys (V2CreatePushSubscriptionBodyKeys):
        filters (V2CreatePushSubscriptionBodyFilters | Unset):
    """

    endpoint: str
    keys: V2CreatePushSubscriptionBodyKeys
    filters: V2CreatePushSubscriptionBodyFilters | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        endpoint = self.endpoint

        keys = self.keys.to_dict()

        filters: dict[str, Any] | Unset = UNSET
        if not isinstance(self.filters, Unset):
            filters = self.filters.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "endpoint": endpoint,
                "keys": keys,
            }
        )
        if filters is not UNSET:
            field_dict["filters"] = filters

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.v2_create_push_subscription_body_filters import (
            V2CreatePushSubscriptionBodyFilters,
        )
        from ..models.v2_create_push_subscription_body_keys import (
            V2CreatePushSubscriptionBodyKeys,
        )

        d = dict(src_dict)
        endpoint = d.pop("endpoint")

        keys = V2CreatePushSubscriptionBodyKeys.from_dict(d.pop("keys"))

        _filters = d.pop("filters", UNSET)
        filters: V2CreatePushSubscriptionBodyFilters | Unset
        if isinstance(_filters, Unset):
            filters = UNSET
        else:
            filters = V2CreatePushSubscriptionBodyFilters.from_dict(_filters)

        v2_create_push_subscription_body = cls(
            endpoint=endpoint,
            keys=keys,
            filters=filters,
        )

        v2_create_push_subscription_body.additional_properties = d
        return v2_create_push_subscription_body

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
