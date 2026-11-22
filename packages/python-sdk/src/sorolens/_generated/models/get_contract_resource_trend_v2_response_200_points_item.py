from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="GetContractResourceTrendV2Response200PointsItem")


@_attrs_define
class GetContractResourceTrendV2Response200PointsItem:
    """
    Attributes:
        date (str):
        avg_cpu_insn (float):
        avg_mem_byte (float):
        avg_fee (float):
        count (int):
    """

    date: str
    avg_cpu_insn: float
    avg_mem_byte: float
    avg_fee: float
    count: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        date = self.date

        avg_cpu_insn = self.avg_cpu_insn

        avg_mem_byte = self.avg_mem_byte

        avg_fee = self.avg_fee

        count = self.count

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "date": date,
                "avg_cpu_insn": avg_cpu_insn,
                "avg_mem_byte": avg_mem_byte,
                "avg_fee": avg_fee,
                "count": count,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        date = d.pop("date")

        avg_cpu_insn = d.pop("avg_cpu_insn")

        avg_mem_byte = d.pop("avg_mem_byte")

        avg_fee = d.pop("avg_fee")

        count = d.pop("count")

        get_contract_resource_trend_v2_response_200_points_item = cls(
            date=date,
            avg_cpu_insn=avg_cpu_insn,
            avg_mem_byte=avg_mem_byte,
            avg_fee=avg_fee,
            count=count,
        )

        get_contract_resource_trend_v2_response_200_points_item.additional_properties = d
        return get_contract_resource_trend_v2_response_200_points_item

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
