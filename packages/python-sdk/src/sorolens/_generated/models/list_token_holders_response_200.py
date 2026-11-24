from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.list_token_holders_response_200_holders_item import (
        ListTokenHoldersResponse200HoldersItem,
    )


T = TypeVar("T", bound="ListTokenHoldersResponse200")


@_attrs_define
class ListTokenHoldersResponse200:
    """
    Attributes:
        holders (list[ListTokenHoldersResponse200HoldersItem]):
        next_cursor (str):
    """

    holders: list[ListTokenHoldersResponse200HoldersItem]
    next_cursor: str
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        holders = []
        for holders_item_data in self.holders:
            holders_item = holders_item_data.to_dict()
            holders.append(holders_item)

        next_cursor = self.next_cursor

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "holders": holders,
                "next_cursor": next_cursor,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.list_token_holders_response_200_holders_item import (
            ListTokenHoldersResponse200HoldersItem,
        )

        d = dict(src_dict)
        holders = []
        _holders = d.pop("holders")
        for holders_item_data in _holders:
            holders_item = ListTokenHoldersResponse200HoldersItem.from_dict(
                holders_item_data
            )

            holders.append(holders_item)

        next_cursor = d.pop("next_cursor")

        list_token_holders_response_200 = cls(
            holders=holders,
            next_cursor=next_cursor,
        )

        list_token_holders_response_200.additional_properties = d
        return list_token_holders_response_200

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
