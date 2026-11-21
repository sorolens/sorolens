from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.usage_timeline_point_by_group import UsageTimelinePointByGroup


T = TypeVar("T", bound="UsageTimelinePoint")


@_attrs_define
class UsageTimelinePoint:
    """
    Attributes:
        date (str):  Example: 2026-11-20.
        total_requests (int):  Example: 125.
        by_group (UsageTimelinePointByGroup):
    """

    date: str
    total_requests: int
    by_group: UsageTimelinePointByGroup
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        date = self.date

        total_requests = self.total_requests

        by_group = self.by_group.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "date": date,
                "total_requests": total_requests,
                "by_group": by_group,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.usage_timeline_point_by_group import UsageTimelinePointByGroup

        d = dict(src_dict)
        date = d.pop("date")

        total_requests = d.pop("total_requests")

        by_group = UsageTimelinePointByGroup.from_dict(d.pop("by_group"))

        usage_timeline_point = cls(
            date=date,
            total_requests=total_requests,
            by_group=by_group,
        )

        usage_timeline_point.additional_properties = d
        return usage_timeline_point

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
