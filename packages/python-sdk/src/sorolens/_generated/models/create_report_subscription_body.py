from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.create_report_subscription_body_frequency import (
    CreateReportSubscriptionBodyFrequency,
)
from ..types import UNSET, Unset

T = TypeVar("T", bound="CreateReportSubscriptionBody")


@_attrs_define
class CreateReportSubscriptionBody:
    """
    Attributes:
        email (str):
        frequency (CreateReportSubscriptionBodyFrequency):
        day_of_week (int | Unset): 0=Sunday..6=Saturday; weekly digests only.
    """

    email: str
    frequency: CreateReportSubscriptionBodyFrequency
    day_of_week: int | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        email = self.email

        frequency = self.frequency.value

        day_of_week = self.day_of_week

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "email": email,
                "frequency": frequency,
            }
        )
        if day_of_week is not UNSET:
            field_dict["day_of_week"] = day_of_week

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        email = d.pop("email")

        frequency = CreateReportSubscriptionBodyFrequency(d.pop("frequency"))

        day_of_week = d.pop("day_of_week", UNSET)

        create_report_subscription_body = cls(
            email=email,
            frequency=frequency,
            day_of_week=day_of_week,
        )

        create_report_subscription_body.additional_properties = d
        return create_report_subscription_body

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
