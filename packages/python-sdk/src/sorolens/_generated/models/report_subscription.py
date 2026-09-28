from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.report_subscription_frequency import ReportSubscriptionFrequency

T = TypeVar("T", bound="ReportSubscription")


@_attrs_define
class ReportSubscription:
    """
    Attributes:
        id (str):
        email (str):
        frequency (ReportSubscriptionFrequency):
        day_of_week (int):
        created_at (datetime.datetime):
    """

    id: str
    email: str
    frequency: ReportSubscriptionFrequency
    day_of_week: int
    created_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = self.id

        email = self.email

        frequency = self.frequency.value

        day_of_week = self.day_of_week

        created_at = self.created_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "email": email,
                "frequency": frequency,
                "day_of_week": day_of_week,
                "created_at": created_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = d.pop("id")

        email = d.pop("email")

        frequency = ReportSubscriptionFrequency(d.pop("frequency"))

        day_of_week = d.pop("day_of_week")

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        report_subscription = cls(
            id=id,
            email=email,
            frequency=frequency,
            day_of_week=day_of_week,
            created_at=created_at,
        )

        report_subscription.additional_properties = d
        return report_subscription

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
