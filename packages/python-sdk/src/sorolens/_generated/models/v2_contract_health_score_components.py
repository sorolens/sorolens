from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="V2ContractHealthScoreComponents")


@_attrs_define
class V2ContractHealthScoreComponents:
    """
    Attributes:
        uptime (int):
        error_rate (int):
        performance (int):
        storage_ttl (int):
    """

    uptime: int
    error_rate: int
    performance: int
    storage_ttl: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        uptime = self.uptime

        error_rate = self.error_rate

        performance = self.performance

        storage_ttl = self.storage_ttl

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "uptime": uptime,
                "error_rate": error_rate,
                "performance": performance,
                "storage_ttl": storage_ttl,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        uptime = d.pop("uptime")

        error_rate = d.pop("error_rate")

        performance = d.pop("performance")

        storage_ttl = d.pop("storage_ttl")

        v2_contract_health_score_components = cls(
            uptime=uptime,
            error_rate=error_rate,
            performance=performance,
            storage_ttl=storage_ttl,
        )

        v2_contract_health_score_components.additional_properties = d
        return v2_contract_health_score_components

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
