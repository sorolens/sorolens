from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="ContractValidationResult")


@_attrs_define
class ContractValidationResult:
    """
    Attributes:
        valid (bool):
        contract_id (str):
        network (str):
        already_tracked (bool):
        label (None | str):
        reason (None | str):
    """

    valid: bool
    contract_id: str
    network: str
    already_tracked: bool
    label: None | str
    reason: None | str
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        valid = self.valid

        contract_id = self.contract_id

        network = self.network

        already_tracked = self.already_tracked

        label: None | str
        label = self.label

        reason: None | str
        reason = self.reason

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "valid": valid,
                "contract_id": contract_id,
                "network": network,
                "already_tracked": already_tracked,
                "label": label,
                "reason": reason,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        valid = d.pop("valid")

        contract_id = d.pop("contract_id")

        network = d.pop("network")

        already_tracked = d.pop("already_tracked")

        def _parse_label(data: object) -> None | str:
            if data is None:
                return data
            return cast(None | str, data)

        label = _parse_label(d.pop("label"))

        def _parse_reason(data: object) -> None | str:
            if data is None:
                return data
            return cast(None | str, data)

        reason = _parse_reason(d.pop("reason"))

        contract_validation_result = cls(
            valid=valid,
            contract_id=contract_id,
            network=network,
            already_tracked=already_tracked,
            label=label,
            reason=reason,
        )

        contract_validation_result.additional_properties = d
        return contract_validation_result

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
