from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="GroupStats")


@_attrs_define
class GroupStats:
    """
    Attributes:
        group_id (UUID):
        contract_count (int):
        event_count (int):
        invocation_count (int):
        storage_entry_count (int):
        average_health_score (float): Mean cached health score across members that have one; 0 when none do.
    """

    group_id: UUID
    contract_count: int
    event_count: int
    invocation_count: int
    storage_entry_count: int
    average_health_score: float
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        group_id = str(self.group_id)

        contract_count = self.contract_count

        event_count = self.event_count

        invocation_count = self.invocation_count

        storage_entry_count = self.storage_entry_count

        average_health_score = self.average_health_score

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "group_id": group_id,
                "contract_count": contract_count,
                "event_count": event_count,
                "invocation_count": invocation_count,
                "storage_entry_count": storage_entry_count,
                "average_health_score": average_health_score,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        group_id = UUID(d.pop("group_id"))

        contract_count = d.pop("contract_count")

        event_count = d.pop("event_count")

        invocation_count = d.pop("invocation_count")

        storage_entry_count = d.pop("storage_entry_count")

        average_health_score = d.pop("average_health_score")

        group_stats = cls(
            group_id=group_id,
            contract_count=contract_count,
            event_count=event_count,
            invocation_count=invocation_count,
            storage_entry_count=storage_entry_count,
            average_health_score=average_health_score,
        )

        group_stats.additional_properties = d
        return group_stats

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
