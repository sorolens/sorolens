from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from typing import cast
import datetime






T = TypeVar("T", bound="WatchlistItem")



@_attrs_define
class WatchlistItem:
    """ 
        Attributes:
            contract_id (str):
            added_at (datetime.datetime):
     """

    contract_id: str
    added_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        added_at = self.added_at.isoformat()


        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
            "contract_id": contract_id,
            "added_at": added_at,
        })

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        added_at = datetime.datetime.fromisoformat(d.pop("added_at"))




        watchlist_item = cls(
            contract_id=contract_id,
            added_at=added_at,
        )


        watchlist_item.additional_properties = d
        return watchlist_item

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
