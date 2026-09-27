from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.v2_alert import V2Alert
    from ..models.v2_pagination import V2Pagination


T = TypeVar("T", bound="V2AlertList")


@_attrs_define
class V2AlertList:
    """
    Attributes:
        data (list[V2Alert]):
        pagination (V2Pagination):
    """

    data: list[V2Alert]
    pagination: V2Pagination
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        data = []
        for data_item_data in self.data:
            data_item = data_item_data.to_dict()
            data.append(data_item)

        pagination = self.pagination.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "data": data,
                "pagination": pagination,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.v2_alert import V2Alert
        from ..models.v2_pagination import V2Pagination

        d = dict(src_dict)
        data = []
        _data = d.pop("data")
        for data_item_data in _data:
            data_item = V2Alert.from_dict(data_item_data)

            data.append(data_item)

        pagination = V2Pagination.from_dict(d.pop("pagination"))

        v2_alert_list = cls(
            data=data,
            pagination=pagination,
        )

        v2_alert_list.additional_properties = d
        return v2_alert_list

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
