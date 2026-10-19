from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.invocation_window_stats import InvocationWindowStats


T = TypeVar("T", bound="ContractInvocationStatsWindows")


@_attrs_define
class ContractInvocationStatsWindows:
    """
    Attributes:
        field_24h (InvocationWindowStats): Fees are measured in stroops, CPU in instructions, and failure_rate is a
            ratio from 0 to 1.
        field_7d (InvocationWindowStats): Fees are measured in stroops, CPU in instructions, and failure_rate is a ratio
            from 0 to 1.
        field_30d (InvocationWindowStats): Fees are measured in stroops, CPU in instructions, and failure_rate is a
            ratio from 0 to 1.
    """

    field_24h: InvocationWindowStats
    field_7d: InvocationWindowStats
    field_30d: InvocationWindowStats
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        field_24h = self.field_24h.to_dict()

        field_7d = self.field_7d.to_dict()

        field_30d = self.field_30d.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "24h": field_24h,
                "7d": field_7d,
                "30d": field_30d,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.invocation_window_stats import InvocationWindowStats

        d = dict(src_dict)
        field_24h = InvocationWindowStats.from_dict(d.pop("24h"))

        field_7d = InvocationWindowStats.from_dict(d.pop("7d"))

        field_30d = InvocationWindowStats.from_dict(d.pop("30d"))

        contract_invocation_stats_windows = cls(
            field_24h=field_24h,
            field_7d=field_7d,
            field_30d=field_30d,
        )

        contract_invocation_stats_windows.additional_properties = d
        return contract_invocation_stats_windows

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
