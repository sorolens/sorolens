from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.maintenance_window import MaintenanceWindow


T = TypeVar("T", bound="ListMaintenanceWindowsResponse200")


@_attrs_define
class ListMaintenanceWindowsResponse200:
    """
    Attributes:
        maintenance_windows (list[MaintenanceWindow]):
    """

    maintenance_windows: list[MaintenanceWindow]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        maintenance_windows = []
        for maintenance_windows_item_data in self.maintenance_windows:
            maintenance_windows_item = maintenance_windows_item_data.to_dict()
            maintenance_windows.append(maintenance_windows_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "maintenance_windows": maintenance_windows,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.maintenance_window import MaintenanceWindow

        d = dict(src_dict)
        maintenance_windows = []
        _maintenance_windows = d.pop("maintenance_windows")
        for maintenance_windows_item_data in _maintenance_windows:
            maintenance_windows_item = MaintenanceWindow.from_dict(
                maintenance_windows_item_data
            )

            maintenance_windows.append(maintenance_windows_item)

        list_maintenance_windows_response_200 = cls(
            maintenance_windows=maintenance_windows,
        )

        list_maintenance_windows_response_200.additional_properties = d
        return list_maintenance_windows_response_200

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
