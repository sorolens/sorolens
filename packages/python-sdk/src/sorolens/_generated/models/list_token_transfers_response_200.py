from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.list_token_transfers_response_200_transfers_item import (
        ListTokenTransfersResponse200TransfersItem,
    )


T = TypeVar("T", bound="ListTokenTransfersResponse200")


@_attrs_define
class ListTokenTransfersResponse200:
    """
    Attributes:
        transfers (list[ListTokenTransfersResponse200TransfersItem]):
        next_cursor (str):
    """

    transfers: list[ListTokenTransfersResponse200TransfersItem]
    next_cursor: str
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        transfers = []
        for transfers_item_data in self.transfers:
            transfers_item = transfers_item_data.to_dict()
            transfers.append(transfers_item)

        next_cursor = self.next_cursor

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "transfers": transfers,
                "next_cursor": next_cursor,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.list_token_transfers_response_200_transfers_item import (
            ListTokenTransfersResponse200TransfersItem,
        )

        d = dict(src_dict)
        transfers = []
        _transfers = d.pop("transfers")
        for transfers_item_data in _transfers:
            transfers_item = ListTokenTransfersResponse200TransfersItem.from_dict(
                transfers_item_data
            )

            transfers.append(transfers_item)

        next_cursor = d.pop("next_cursor")

        list_token_transfers_response_200 = cls(
            transfers=transfers,
            next_cursor=next_cursor,
        )

        list_token_transfers_response_200.additional_properties = d
        return list_token_transfers_response_200

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
