from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.update_maintenance_window_request_recurrence import (
    UpdateMaintenanceWindowRequestRecurrence,
)
from ..types import UNSET, Unset

T = TypeVar("T", bound="UpdateMaintenanceWindowRequest")


@_attrs_define
class UpdateMaintenanceWindowRequest:
    """
    Attributes:
        name (str):
        start_time (datetime.datetime):
        end_time (datetime.datetime):
        timezone (str | Unset):  Default: 'UTC'.
        recurrence (UpdateMaintenanceWindowRequestRecurrence | Unset):  Default:
            UpdateMaintenanceWindowRequestRecurrence.NONE.
        suppress_notifications (bool | Unset):  Default: True.
    """

    name: str
    start_time: datetime.datetime
    end_time: datetime.datetime
    timezone: str | Unset = "UTC"
    recurrence: UpdateMaintenanceWindowRequestRecurrence | Unset = (
        UpdateMaintenanceWindowRequestRecurrence.NONE
    )
    suppress_notifications: bool | Unset = True
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        start_time = self.start_time.isoformat()

        end_time = self.end_time.isoformat()

        timezone = self.timezone

        recurrence: str | Unset = UNSET
        if not isinstance(self.recurrence, Unset):
            recurrence = self.recurrence.value

        suppress_notifications = self.suppress_notifications

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "name": name,
                "start_time": start_time,
                "end_time": end_time,
            }
        )
        if timezone is not UNSET:
            field_dict["timezone"] = timezone
        if recurrence is not UNSET:
            field_dict["recurrence"] = recurrence
        if suppress_notifications is not UNSET:
            field_dict["suppress_notifications"] = suppress_notifications

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name")

        start_time = datetime.datetime.fromisoformat(d.pop("start_time"))

        end_time = datetime.datetime.fromisoformat(d.pop("end_time"))

        timezone = d.pop("timezone", UNSET)

        _recurrence = d.pop("recurrence", UNSET)
        recurrence: UpdateMaintenanceWindowRequestRecurrence | Unset
        if isinstance(_recurrence, Unset):
            recurrence = UNSET
        else:
            recurrence = UpdateMaintenanceWindowRequestRecurrence(_recurrence)

        suppress_notifications = d.pop("suppress_notifications", UNSET)

        update_maintenance_window_request = cls(
            name=name,
            start_time=start_time,
            end_time=end_time,
            timezone=timezone,
            recurrence=recurrence,
            suppress_notifications=suppress_notifications,
        )

        update_maintenance_window_request.additional_properties = d
        return update_maintenance_window_request

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
