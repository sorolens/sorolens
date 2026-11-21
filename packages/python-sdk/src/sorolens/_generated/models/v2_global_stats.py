from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

T = TypeVar("T", bound="V2GlobalStats")


@_attrs_define
class V2GlobalStats:
    """
    Attributes:
        tracked_contracts (int):
        total_events (int):
        total_invocations (int):
        total_storage_entries (int):
        approximate (bool | Unset): True when large table counts are estimated rather than exact.
    """

    tracked_contracts: int
    total_events: int
    total_invocations: int
    total_storage_entries: int
    approximate: bool | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        tracked_contracts = self.tracked_contracts

        total_events = self.total_events

        total_invocations = self.total_invocations

        total_storage_entries = self.total_storage_entries

        approximate = self.approximate

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "tracked_contracts": tracked_contracts,
                "total_events": total_events,
                "total_invocations": total_invocations,
                "total_storage_entries": total_storage_entries,
            }
        )
        if approximate is not UNSET:
            field_dict["approximate"] = approximate

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        tracked_contracts = d.pop("tracked_contracts")

        total_events = d.pop("total_events")

        total_invocations = d.pop("total_invocations")

        total_storage_entries = d.pop("total_storage_entries")

        approximate = d.pop("approximate", UNSET)

        v2_global_stats = cls(
            tracked_contracts=tracked_contracts,
            total_events=total_events,
            total_invocations=total_invocations,
            total_storage_entries=total_storage_entries,
            approximate=approximate,
        )

        v2_global_stats.additional_properties = d
        return v2_global_stats

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
