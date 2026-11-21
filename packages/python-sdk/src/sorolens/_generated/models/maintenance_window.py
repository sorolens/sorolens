from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.maintenance_window_recurrence import MaintenanceWindowRecurrence
from ..models.maintenance_window_scope import MaintenanceWindowScope
from ..types import UNSET, Unset

T = TypeVar("T", bound="MaintenanceWindow")


@_attrs_define
class MaintenanceWindow:
    """
    Attributes:
        id (int):
        name (str):
        scope (MaintenanceWindowScope):
        start_time (datetime.datetime):
        end_time (datetime.datetime):
        timezone (str):
        recurrence (MaintenanceWindowRecurrence):
        suppress_notifications (bool):
        target_id (str | Unset):
        created_by (str | Unset):
        created_at (datetime.datetime | Unset):
        updated_at (datetime.datetime | Unset):
    """

    id: int
    name: str
    scope: MaintenanceWindowScope
    start_time: datetime.datetime
    end_time: datetime.datetime
    timezone: str
    recurrence: MaintenanceWindowRecurrence
    suppress_notifications: bool
    target_id: str | Unset = UNSET
    created_by: str | Unset = UNSET
    created_at: datetime.datetime | Unset = UNSET
    updated_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = self.id

        name = self.name

        scope = self.scope.value

        start_time = self.start_time.isoformat()

        end_time = self.end_time.isoformat()

        timezone = self.timezone

        recurrence = self.recurrence.value

        suppress_notifications = self.suppress_notifications

        target_id = self.target_id

        created_by = self.created_by

        created_at: str | Unset = UNSET
        if not isinstance(self.created_at, Unset):
            created_at = self.created_at.isoformat()

        updated_at: str | Unset = UNSET
        if not isinstance(self.updated_at, Unset):
            updated_at = self.updated_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "name": name,
                "scope": scope,
                "start_time": start_time,
                "end_time": end_time,
                "timezone": timezone,
                "recurrence": recurrence,
                "suppress_notifications": suppress_notifications,
            }
        )
        if target_id is not UNSET:
            field_dict["target_id"] = target_id
        if created_by is not UNSET:
            field_dict["created_by"] = created_by
        if created_at is not UNSET:
            field_dict["created_at"] = created_at
        if updated_at is not UNSET:
            field_dict["updated_at"] = updated_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = d.pop("id")

        name = d.pop("name")

        scope = MaintenanceWindowScope(d.pop("scope"))

        start_time = datetime.datetime.fromisoformat(d.pop("start_time"))

        end_time = datetime.datetime.fromisoformat(d.pop("end_time"))

        timezone = d.pop("timezone")

        recurrence = MaintenanceWindowRecurrence(d.pop("recurrence"))

        suppress_notifications = d.pop("suppress_notifications")

        target_id = d.pop("target_id", UNSET)

        created_by = d.pop("created_by", UNSET)

        _created_at = d.pop("created_at", UNSET)
        created_at: datetime.datetime | Unset
        if isinstance(_created_at, Unset):
            created_at = UNSET
        else:
            created_at = datetime.datetime.fromisoformat(_created_at)

        _updated_at = d.pop("updated_at", UNSET)
        updated_at: datetime.datetime | Unset
        if isinstance(_updated_at, Unset):
            updated_at = UNSET
        else:
            updated_at = datetime.datetime.fromisoformat(_updated_at)

        maintenance_window = cls(
            id=id,
            name=name,
            scope=scope,
            start_time=start_time,
            end_time=end_time,
            timezone=timezone,
            recurrence=recurrence,
            suppress_notifications=suppress_notifications,
            target_id=target_id,
            created_by=created_by,
            created_at=created_at,
            updated_at=updated_at,
        )

        maintenance_window.additional_properties = d
        return maintenance_window

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
