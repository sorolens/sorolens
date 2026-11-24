from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="ListTokenHoldersResponse200HoldersItem")


@_attrs_define
class ListTokenHoldersResponse200HoldersItem:
    """
    Attributes:
        holder (str):
        balance (str): Token balance as a decimal string (numeric(39,0)).
        updated_at (datetime.datetime):
    """

    holder: str
    balance: str
    updated_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        holder = self.holder

        balance = self.balance

        updated_at = self.updated_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "holder": holder,
                "balance": balance,
                "updated_at": updated_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        holder = d.pop("holder")

        balance = d.pop("balance")

        updated_at = datetime.datetime.fromisoformat(d.pop("updated_at"))

        list_token_holders_response_200_holders_item = cls(
            holder=holder,
            balance=balance,
            updated_at=updated_at,
        )

        list_token_holders_response_200_holders_item.additional_properties = d
        return list_token_holders_response_200_holders_item

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
