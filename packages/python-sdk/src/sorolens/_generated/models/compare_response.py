from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.compare_response_window import CompareResponseWindow

if TYPE_CHECKING:
    from ..models.compare_contract_entry import CompareContractEntry


T = TypeVar("T", bound="CompareResponse")


@_attrs_define
class CompareResponse:
    """
    Attributes:
        window (CompareResponseWindow):
        contracts (list[CompareContractEntry]):
    """

    window: CompareResponseWindow
    contracts: list[CompareContractEntry]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        window = self.window.value

        contracts = []
        for contracts_item_data in self.contracts:
            contracts_item = contracts_item_data.to_dict()
            contracts.append(contracts_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "window": window,
                "contracts": contracts,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.compare_contract_entry import (
            CompareContractEntry,
        )

        d = dict(src_dict)
        window = CompareResponseWindow(d.pop("window"))

        contracts = []
        _contracts = d.pop("contracts")
        for contracts_item_data in _contracts:
            contracts_item = CompareContractEntry.from_dict(contracts_item_data)

            contracts.append(contracts_item)

        compare_response = cls(
            window=window,
            contracts=contracts,
        )

        compare_response.additional_properties = d
        return compare_response

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
