from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.uptime_result_window import UptimeResultWindow

T = TypeVar("T", bound="UptimeResult")


@_attrs_define
class UptimeResult:
    """
    Attributes:
        contract_id (str): The contract ID this uptime figure applies to.
        window (UptimeResultWindow): The time window over which uptime was computed.
        uptime_pct (float): Uptime percentage with up to two decimal places.
            Computed as healthy_checks / total_checks × 100.
            Returns 0 when no health checks were recorded in the window.
    """

    contract_id: str
    window: UptimeResultWindow
    uptime_pct: float
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        window = self.window.value

        uptime_pct = self.uptime_pct

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "contract_id": contract_id,
                "window": window,
                "uptime_pct": uptime_pct,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        window = UptimeResultWindow(d.pop("window"))

        uptime_pct = d.pop("uptime_pct")

        uptime_result = cls(
            contract_id=contract_id,
            window=window,
            uptime_pct=uptime_pct,
        )

        uptime_result.additional_properties = d
        return uptime_result

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
