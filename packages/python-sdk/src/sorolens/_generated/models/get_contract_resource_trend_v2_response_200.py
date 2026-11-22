from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.get_contract_resource_trend_v2_response_200_points_item import (
        GetContractResourceTrendV2Response200PointsItem,
    )


T = TypeVar("T", bound="GetContractResourceTrendV2Response200")


@_attrs_define
class GetContractResourceTrendV2Response200:
    """
    Attributes:
        days (int):
        points (list[GetContractResourceTrendV2Response200PointsItem]):
    """

    days: int
    points: list[GetContractResourceTrendV2Response200PointsItem]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        days = self.days

        points = []
        for points_item_data in self.points:
            points_item = points_item_data.to_dict()
            points.append(points_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "days": days,
                "points": points,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.get_contract_resource_trend_v2_response_200_points_item import (
            GetContractResourceTrendV2Response200PointsItem,
        )

        d = dict(src_dict)
        days = d.pop("days")

        points = []
        _points = d.pop("points")
        for points_item_data in _points:
            points_item = GetContractResourceTrendV2Response200PointsItem.from_dict(
                points_item_data
            )

            points.append(points_item)

        get_contract_resource_trend_v2_response_200 = cls(
            days=days,
            points=points,
        )

        get_contract_resource_trend_v2_response_200.additional_properties = d
        return get_contract_resource_trend_v2_response_200

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
