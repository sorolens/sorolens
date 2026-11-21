from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.create_maintenance_window_request_recurrence import (
    CreateMaintenanceWindowRequestRecurrence,
)
from ..models.create_maintenance_window_request_scope import (
    CreateMaintenanceWindowRequestScope,
)
from ..types import UNSET, Unset

T = TypeVar("T", bound="CreateMaintenanceWindowRequest")


@_attrs_define
class CreateMaintenanceWindowRequest:
    """
    Attributes:
        name (str):
        start_time (datetime.datetime):
        end_time (datetime.datetime):
        scope (CreateMaintenanceWindowRequestScope | Unset):  Default: CreateMaintenanceWindowRequestScope.GLOBAL.
        target_id (str | Unset):
        timezone (str | Unset):  Default: 'UTC'.
        recurrence (CreateMaintenanceWindowRequestRecurrence | Unset):  Default:
            CreateMaintenanceWindowRequestRecurrence.NONE.
        suppress_notifications (bool | Unset):  Default: True.
    """

    name: str
    start_time: datetime.datetime
    end_time: datetime.datetime
    scope: CreateMaintenanceWindowRequestScope | Unset = (
        CreateMaintenanceWindowRequestScope.GLOBAL
    )
    target_id: str | Unset = UNSET
    timezone: str | Unset = "UTC"
    recurrence: CreateMaintenanceWindowRequestRecurrence | Unset = (
        CreateMaintenanceWindowRequestRecurrence.NONE
    )
    suppress_notifications: bool | Unset = True
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        start_time = self.start_time.isoformat()

        end_time = self.end_time.isoformat()

        scope: str | Unset = UNSET
        if not isinstance(self.scope, Unset):
            scope = self.scope.value

        target_id = self.target_id

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
        if scope is not UNSET:
            field_dict["scope"] = scope
        if target_id is not UNSET:
            field_dict["target_id"] = target_id
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

        _scope = d.pop("scope", UNSET)
        scope: CreateMaintenanceWindowRequestScope | Unset
        if isinstance(_scope, Unset):
            scope = UNSET
        else:
            scope = CreateMaintenanceWindowRequestScope(_scope)

        target_id = d.pop("target_id", UNSET)

        timezone = d.pop("timezone", UNSET)

        _recurrence = d.pop("recurrence", UNSET)
        recurrence: CreateMaintenanceWindowRequestRecurrence | Unset
        if isinstance(_recurrence, Unset):
            recurrence = UNSET
        else:
            recurrence = CreateMaintenanceWindowRequestRecurrence(_recurrence)

        suppress_notifications = d.pop("suppress_notifications", UNSET)

        create_maintenance_window_request = cls(
            name=name,
            start_time=start_time,
            end_time=end_time,
            scope=scope,
            target_id=target_id,
            timezone=timezone,
            recurrence=recurrence,
            suppress_notifications=suppress_notifications,
        )

        create_maintenance_window_request.additional_properties = d
        return create_maintenance_window_request

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
