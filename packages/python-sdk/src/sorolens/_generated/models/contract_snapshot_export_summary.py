from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

T = TypeVar("T", bound="ContractSnapshotExportSummary")


@_attrs_define
class ContractSnapshotExportSummary:
    """
    Attributes:
        storage_count (int):
        event_count (int):
        first_tracked_ledger (int):
        last_event_id (str | Unset): Omitted when the contract has no events.
        last_event_ledger (int | Unset):
        storage_truncated (bool | Unset): Present and true only when storage hit the 1000-entry cap.
    """

    storage_count: int
    event_count: int
    first_tracked_ledger: int
    last_event_id: str | Unset = UNSET
    last_event_ledger: int | Unset = UNSET
    storage_truncated: bool | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        storage_count = self.storage_count

        event_count = self.event_count

        first_tracked_ledger = self.first_tracked_ledger

        last_event_id = self.last_event_id

        last_event_ledger = self.last_event_ledger

        storage_truncated = self.storage_truncated

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "storage_count": storage_count,
                "event_count": event_count,
                "first_tracked_ledger": first_tracked_ledger,
            }
        )
        if last_event_id is not UNSET:
            field_dict["last_event_id"] = last_event_id
        if last_event_ledger is not UNSET:
            field_dict["last_event_ledger"] = last_event_ledger
        if storage_truncated is not UNSET:
            field_dict["storage_truncated"] = storage_truncated

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        storage_count = d.pop("storage_count")

        event_count = d.pop("event_count")

        first_tracked_ledger = d.pop("first_tracked_ledger")

        last_event_id = d.pop("last_event_id", UNSET)

        last_event_ledger = d.pop("last_event_ledger", UNSET)

        storage_truncated = d.pop("storage_truncated", UNSET)

        contract_snapshot_export_summary = cls(
            storage_count=storage_count,
            event_count=event_count,
            first_tracked_ledger=first_tracked_ledger,
            last_event_id=last_event_id,
            last_event_ledger=last_event_ledger,
            storage_truncated=storage_truncated,
        )

        contract_snapshot_export_summary.additional_properties = d
        return contract_snapshot_export_summary

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
