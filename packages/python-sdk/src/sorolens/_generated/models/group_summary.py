from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.group_stats import GroupStats


T = TypeVar("T", bound="GroupSummary")


@_attrs_define
class GroupSummary:
    """
    Attributes:
        id (UUID):
        owner_id (str):
        name (str):
        created_at (datetime.datetime):
        stats (GroupStats):
    """

    id: UUID
    owner_id: str
    name: str
    created_at: datetime.datetime
    stats: GroupStats
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        owner_id = self.owner_id

        name = self.name

        created_at = self.created_at.isoformat()

        stats = self.stats.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "owner_id": owner_id,
                "name": name,
                "created_at": created_at,
                "stats": stats,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.group_stats import GroupStats

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        owner_id = d.pop("owner_id")

        name = d.pop("name")

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        stats = GroupStats.from_dict(d.pop("stats"))

        group_summary = cls(
            id=id,
            owner_id=owner_id,
            name=name,
            created_at=created_at,
            stats=stats,
        )

        group_summary.additional_properties = d
        return group_summary

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
