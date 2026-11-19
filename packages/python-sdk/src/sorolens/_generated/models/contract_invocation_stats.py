from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.contract_invocation_stats_windows import (
        ContractInvocationStatsWindows,
    )


T = TypeVar("T", bound="ContractInvocationStats")


@_attrs_define
class ContractInvocationStats:
    """
    Attributes:
        windows (ContractInvocationStatsWindows):
    """

    windows: ContractInvocationStatsWindows
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        windows = self.windows.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "windows": windows,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.contract_invocation_stats_windows import (
            ContractInvocationStatsWindows,
        )

        d = dict(src_dict)
        windows = ContractInvocationStatsWindows.from_dict(d.pop("windows"))

        contract_invocation_stats = cls(
            windows=windows,
        )

        contract_invocation_stats.additional_properties = d
        return contract_invocation_stats

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
