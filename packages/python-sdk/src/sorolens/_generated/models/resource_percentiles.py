from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.percentile_values import PercentileValues


T = TypeVar("T", bound="ResourcePercentiles")


@_attrs_define
class ResourcePercentiles:
    """Per-invocation resource usage percentiles over the requested window. Returned by the contract stats endpoint; every
    value is 0 when the window has no invocations.

        Attributes:
            cpu_insn (PercentileValues):
            mem_byte (PercentileValues):
            fee (PercentileValues):
    """

    cpu_insn: PercentileValues
    mem_byte: PercentileValues
    fee: PercentileValues
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        cpu_insn = self.cpu_insn.to_dict()

        mem_byte = self.mem_byte.to_dict()

        fee = self.fee.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "cpu_insn": cpu_insn,
                "mem_byte": mem_byte,
                "fee": fee,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.percentile_values import PercentileValues

        d = dict(src_dict)
        cpu_insn = PercentileValues.from_dict(d.pop("cpu_insn"))

        mem_byte = PercentileValues.from_dict(d.pop("mem_byte"))

        fee = PercentileValues.from_dict(d.pop("fee"))

        resource_percentiles = cls(
            cpu_insn=cpu_insn,
            mem_byte=mem_byte,
            fee=fee,
        )

        resource_percentiles.additional_properties = d
        return resource_percentiles

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
