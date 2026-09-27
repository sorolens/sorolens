from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="ContractEventRate")


@_attrs_define
class ContractEventRate:
    """
    Attributes:
        contract_id (str):
        label (str):
        network (str):
        total (int): Events observed in the window across all buckets.
        per_minute (list[int]): One bucket per minute over the window, oldest first. The length
            always equals the requested minute count, so the series is
            contiguous and a sparkline's x-axis never shifts.
    """

    contract_id: str
    label: str
    network: str
    total: int
    per_minute: list[int]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        label = self.label

        network = self.network

        total = self.total

        per_minute = self.per_minute

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "contract_id": contract_id,
                "label": label,
                "network": network,
                "total": total,
                "per_minute": per_minute,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        label = d.pop("label")

        network = d.pop("network")

        total = d.pop("total")

        per_minute = cast(list[int], d.pop("per_minute"))

        contract_event_rate = cls(
            contract_id=contract_id,
            label=label,
            network=network,
            total=total,
            per_minute=per_minute,
        )

        contract_event_rate.additional_properties = d
        return contract_event_rate

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
