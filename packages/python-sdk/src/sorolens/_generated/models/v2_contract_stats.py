from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="V2ContractStats")


@_attrs_define
class V2ContractStats:
    """
    Attributes:
        event_count (int):
        invocation_count (int):
        storage_count (int):
        last_synced_ledger (int):
        window_event_count (int):
        window_invocation_count (int):
        window_duration (str):
    """

    event_count: int
    invocation_count: int
    storage_count: int
    last_synced_ledger: int
    window_event_count: int
    window_invocation_count: int
    window_duration: str
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        event_count = self.event_count

        invocation_count = self.invocation_count

        storage_count = self.storage_count

        last_synced_ledger = self.last_synced_ledger

        window_event_count = self.window_event_count

        window_invocation_count = self.window_invocation_count

        window_duration = self.window_duration

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "event_count": event_count,
                "invocation_count": invocation_count,
                "storage_count": storage_count,
                "last_synced_ledger": last_synced_ledger,
                "window_event_count": window_event_count,
                "window_invocation_count": window_invocation_count,
                "window_duration": window_duration,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        event_count = d.pop("event_count")

        invocation_count = d.pop("invocation_count")

        storage_count = d.pop("storage_count")

        last_synced_ledger = d.pop("last_synced_ledger")

        window_event_count = d.pop("window_event_count")

        window_invocation_count = d.pop("window_invocation_count")

        window_duration = d.pop("window_duration")

        v2_contract_stats = cls(
            event_count=event_count,
            invocation_count=invocation_count,
            storage_count=storage_count,
            last_synced_ledger=last_synced_ledger,
            window_event_count=window_event_count,
            window_invocation_count=window_invocation_count,
            window_duration=window_duration,
        )

        v2_contract_stats.additional_properties = d
        return v2_contract_stats

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
