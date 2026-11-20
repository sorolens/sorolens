from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.key_usage_summary_by_group import KeyUsageSummaryByGroup
    from ..models.usage_timeline_point import UsageTimelinePoint


T = TypeVar("T", bound="KeyUsageSummary")


@_attrs_define
class KeyUsageSummary:
    """
    Attributes:
        key_id (str):  Example: key_01h7....
        from_ (datetime.datetime):
        to (datetime.datetime):
        total_requests (int):  Example: 1540.
        by_group (KeyUsageSummaryByGroup):
        timeline (list[UsageTimelinePoint]):
        month_used (int):  Example: 1540.
        monthly_quota (int | None | Unset):  Example: 50000.
        month_remaining (int | None | Unset):  Example: 48460.
    """

    key_id: str
    from_: datetime.datetime
    to: datetime.datetime
    total_requests: int
    by_group: KeyUsageSummaryByGroup
    timeline: list[UsageTimelinePoint]
    month_used: int
    monthly_quota: int | None | Unset = UNSET
    month_remaining: int | None | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        key_id = self.key_id

        from_ = self.from_.isoformat()

        to = self.to.isoformat()

        total_requests = self.total_requests

        by_group = self.by_group.to_dict()

        timeline = []
        for timeline_item_data in self.timeline:
            timeline_item = timeline_item_data.to_dict()
            timeline.append(timeline_item)

        month_used = self.month_used

        monthly_quota: int | None | Unset
        if isinstance(self.monthly_quota, Unset):
            monthly_quota = UNSET
        else:
            monthly_quota = self.monthly_quota

        month_remaining: int | None | Unset
        if isinstance(self.month_remaining, Unset):
            month_remaining = UNSET
        else:
            month_remaining = self.month_remaining

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "key_id": key_id,
                "from": from_,
                "to": to,
                "total_requests": total_requests,
                "by_group": by_group,
                "timeline": timeline,
                "month_used": month_used,
            }
        )
        if monthly_quota is not UNSET:
            field_dict["monthly_quota"] = monthly_quota
        if month_remaining is not UNSET:
            field_dict["month_remaining"] = month_remaining

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.key_usage_summary_by_group import KeyUsageSummaryByGroup
        from ..models.usage_timeline_point import UsageTimelinePoint

        d = dict(src_dict)
        key_id = d.pop("key_id")

        from_ = datetime.datetime.fromisoformat(d.pop("from"))

        to = datetime.datetime.fromisoformat(d.pop("to"))

        total_requests = d.pop("total_requests")

        by_group = KeyUsageSummaryByGroup.from_dict(d.pop("by_group"))

        timeline = []
        _timeline = d.pop("timeline")
        for timeline_item_data in _timeline:
            timeline_item = UsageTimelinePoint.from_dict(timeline_item_data)

            timeline.append(timeline_item)

        month_used = d.pop("month_used")

        def _parse_monthly_quota(data: object) -> int | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(int | None | Unset, data)

        monthly_quota = _parse_monthly_quota(d.pop("monthly_quota", UNSET))

        def _parse_month_remaining(data: object) -> int | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(int | None | Unset, data)

        month_remaining = _parse_month_remaining(d.pop("month_remaining", UNSET))

        key_usage_summary = cls(
            key_id=key_id,
            from_=from_,
            to=to,
            total_requests=total_requests,
            by_group=by_group,
            timeline=timeline,
            month_used=month_used,
            monthly_quota=monthly_quota,
            month_remaining=month_remaining,
        )

        key_usage_summary.additional_properties = d
        return key_usage_summary

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
