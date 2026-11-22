from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

T = TypeVar("T", bound="GetAlertCountsResponse200Counts")


@_attrs_define
class GetAlertCountsResponse200Counts:
    """
    Attributes:
        open_ (int | Unset):
        acknowledged (int | Unset):
        snoozed (int | Unset):
        resolved (int | Unset):
    """

    open_: int | Unset = UNSET
    acknowledged: int | Unset = UNSET
    snoozed: int | Unset = UNSET
    resolved: int | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        open_ = self.open_

        acknowledged = self.acknowledged

        snoozed = self.snoozed

        resolved = self.resolved

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if open_ is not UNSET:
            field_dict["open"] = open_
        if acknowledged is not UNSET:
            field_dict["acknowledged"] = acknowledged
        if snoozed is not UNSET:
            field_dict["snoozed"] = snoozed
        if resolved is not UNSET:
            field_dict["resolved"] = resolved

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        open_ = d.pop("open", UNSET)

        acknowledged = d.pop("acknowledged", UNSET)

        snoozed = d.pop("snoozed", UNSET)

        resolved = d.pop("resolved", UNSET)

        get_alert_counts_response_200_counts = cls(
            open_=open_,
            acknowledged=acknowledged,
            snoozed=snoozed,
            resolved=resolved,
        )

        get_alert_counts_response_200_counts.additional_properties = d
        return get_alert_counts_response_200_counts

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
