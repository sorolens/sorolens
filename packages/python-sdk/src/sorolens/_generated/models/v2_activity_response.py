from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.v2_contract_rate import V2ContractRate


T = TypeVar("T", bound="V2ActivityResponse")


@_attrs_define
class V2ActivityResponse:
    """
    Attributes:
        minutes (int):
        window_start (datetime.datetime):
        data (list[V2ContractRate]):
    """

    minutes: int
    window_start: datetime.datetime
    data: list[V2ContractRate]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        minutes = self.minutes

        window_start = self.window_start.isoformat()

        data = []
        for data_item_data in self.data:
            data_item = data_item_data.to_dict()
            data.append(data_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "minutes": minutes,
                "window_start": window_start,
                "data": data,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.v2_contract_rate import V2ContractRate

        d = dict(src_dict)
        minutes = d.pop("minutes")

        window_start = datetime.datetime.fromisoformat(d.pop("window_start"))

        data = []
        _data = d.pop("data")
        for data_item_data in _data:
            data_item = V2ContractRate.from_dict(data_item_data)

            data.append(data_item)

        v2_activity_response = cls(
            minutes=minutes,
            window_start=window_start,
            data=data,
        )

        v2_activity_response.additional_properties = d
        return v2_activity_response

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
