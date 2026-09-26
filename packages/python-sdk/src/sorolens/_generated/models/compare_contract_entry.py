from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.compare_contract_entry_event_volume_item import (
        CompareContractEntryEventVolumeItem,
    )


T = TypeVar("T", bound="CompareContractEntry")


@_attrs_define
class CompareContractEntry:
    """
    Attributes:
        id (str):
        network (str):
        label (str):
        status (str):
        tracked (bool):
        has_data (bool):
        event_count (int):
        invocation_count (int):
        avg_cpu (float):
        avg_fee (float):
        last_synced_ledger (int):
        event_volume (list[CompareContractEntryEventVolumeItem]):
        health_score (int | None | Unset):
        error (str | Unset):
    """

    id: str
    network: str
    label: str
    status: str
    tracked: bool
    has_data: bool
    event_count: int
    invocation_count: int
    avg_cpu: float
    avg_fee: float
    last_synced_ledger: int
    event_volume: list[CompareContractEntryEventVolumeItem]
    health_score: int | None | Unset = UNSET
    error: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = self.id

        network = self.network

        label = self.label

        status = self.status

        tracked = self.tracked

        has_data = self.has_data

        event_count = self.event_count

        invocation_count = self.invocation_count

        avg_cpu = self.avg_cpu

        avg_fee = self.avg_fee

        last_synced_ledger = self.last_synced_ledger

        event_volume = []
        for event_volume_item_data in self.event_volume:
            event_volume_item = event_volume_item_data.to_dict()
            event_volume.append(event_volume_item)

        health_score: int | None | Unset
        if isinstance(self.health_score, Unset):
            health_score = UNSET
        else:
            health_score = self.health_score

        error = self.error

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "network": network,
                "label": label,
                "status": status,
                "tracked": tracked,
                "has_data": has_data,
                "event_count": event_count,
                "invocation_count": invocation_count,
                "avg_cpu": avg_cpu,
                "avg_fee": avg_fee,
                "last_synced_ledger": last_synced_ledger,
                "event_volume": event_volume,
            }
        )
        if health_score is not UNSET:
            field_dict["health_score"] = health_score
        if error is not UNSET:
            field_dict["error"] = error

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.compare_contract_entry_event_volume_item import (
            CompareContractEntryEventVolumeItem,
        )

        d = dict(src_dict)
        id = d.pop("id")

        network = d.pop("network")

        label = d.pop("label")

        status = d.pop("status")

        tracked = d.pop("tracked")

        has_data = d.pop("has_data")

        event_count = d.pop("event_count")

        invocation_count = d.pop("invocation_count")

        avg_cpu = d.pop("avg_cpu")

        avg_fee = d.pop("avg_fee")

        last_synced_ledger = d.pop("last_synced_ledger")

        event_volume = []
        _event_volume = d.pop("event_volume")
        for event_volume_item_data in _event_volume:
            event_volume_item = CompareContractEntryEventVolumeItem.from_dict(
                event_volume_item_data
            )

            event_volume.append(event_volume_item)

        def _parse_health_score(data: object) -> int | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(int | None | Unset, data)

        health_score = _parse_health_score(d.pop("health_score", UNSET))

        error = d.pop("error", UNSET)

        compare_contract_entry = cls(
            id=id,
            network=network,
            label=label,
            status=status,
            tracked=tracked,
            has_data=has_data,
            event_count=event_count,
            invocation_count=invocation_count,
            avg_cpu=avg_cpu,
            avg_fee=avg_fee,
            last_synced_ledger=last_synced_ledger,
            event_volume=event_volume,
            health_score=health_score,
            error=error,
        )

        compare_contract_entry.additional_properties = d
        return compare_contract_entry

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
